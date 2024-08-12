package controller

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const (
	pullInterval      = 5
	routingServerPort = ":5200"
)

type RoutingServer struct {
	etcdClient   *clientv3.Client
	routingTable atomic.Pointer[routingpb.RoutingTable]
	logVersion   atomic.Int64
	ctx          context.Context
	cancelFunc   context.CancelFunc
	loc          *time.Location
	changeLogs   []*routingpb.ChangeRecords
	mu           sync.RWMutex

	consumerpb.UnimplementedConsumerServiceServer
}

func NewRoutingServer() (*RoutingServer, error) {
	server := &RoutingServer{
		changeLogs:   make([]*routingpb.ChangeRecords, changeLogLength, changeLogLength),
		routingTable: atomic.Pointer[routingpb.RoutingTable]{},
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdUri},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	server.etcdClient = client
	ctx, cancel := context.WithCancel(context.Background())
	server.ctx = ctx
	server.cancelFunc = cancel
	return server, nil
}

func (s *RoutingServer) Run() {
	go s.serveGrpc()
	// go s.watchLoop()
	go s.pullLoop()
}

func (s *RoutingServer) Stop() {
	s.cancelFunc()
}

func (s *RoutingServer) serveGrpc() {
	lis, err := net.Listen("tcp", routingServerPort)
	if err != nil {
		mlog.Fatalf("failed to start grpc server: %v", err)
	}

	server := grpc.NewServer()
	consumerpb.RegisterConsumerServiceServer(server, s)
	mlog.Infof("server listening on port: %v", lis.Addr())
	if err := server.Serve(lis); err != nil {
		mlog.Fatalf("failed to serve: %v", err)
	}
}

func (s *RoutingServer) pullLoop() {
	ticker := time.NewTicker(pullInterval * time.Second)
	for {
		select {
		case <-ticker.C:
			if err := s.updateLocalRoutingTable(); err != nil {
				mlog.Errorf("failed to update local routing table: %v", err)
			}
		case <-s.ctx.Done():
			mlog.Info("pull loop stopped")
			return
		}
	}
}

func (s *RoutingServer) updateLocalRoutingTable() error {
	resp, err := s.etcdClient.Get(s.ctx, routingTableKey)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	routingTable := &routingpb.RoutingTable{}
	if err := json.Unmarshal(resp.Kvs[0].Value, routingTable); err != nil {
		return util.ErrorWithPos(err)
	}
	s.routingTable.Store(routingTable)
	mlog.Debug("update get routing table", zap.Any("routing table", routingTable))
	return nil
}

func (s *RoutingServer) watchLoop() {
	watchChan := s.etcdClient.Watch(s.ctx, routingLogPrefix, clientv3.WithPrefix())
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			mlog.Debugf("Received etcd event: %v", event)
			switch event.Type {
			case mvccpb.PUT:
				log := &routingpb.ChangeRecords{}
				if err := json.Unmarshal(event.Kv.Value, log); err != nil {
					mlog.Errorf("failed to unmarshal change log: %v", err)
					break
				}
				if !event.IsCreate() {
					mlog.Warn("change log put again", zap.Any("change log", log))
					break
				}
				s.updateLog(log)
			}
		}
	}
}

func (s *RoutingServer) updateLog(log *routingpb.ChangeRecords) {
	targetVersion := log.GetVersion()
	// 版本号初始化为最新version - 1
	if s.logVersion.Load() == 0 {
		s.logVersion.Store(targetVersion - 1)
	}
	currentVersion := s.logVersion.Load()
	if currentVersion >= targetVersion {
		mlog.Fatal("log version illegal", zap.Any("current log version", currentVersion), zap.Any("target version", targetVersion))
	} else if currentVersion+3 < targetVersion {
		s.logVersion.Store(targetVersion)
		s.clearLog()
		mlog.Info("log version too low, clear all log")
	} else {
		s.logVersion.Add(1)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// 队头出队
	s.changeLogs = s.changeLogs[1:]
	// 队尾入队
	s.changeLogs = append(s.changeLogs, log)
	mlog.Info("updated log", zap.Any("log version", targetVersion), zap.Any("log", log))
}

func (s *RoutingServer) clearLog() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.changeLogs = make([]*routingpb.ChangeRecords, changeLogLength, changeLogLength)
}

func (s *RoutingServer) ConsumerInit(ctx context.Context, req *consumerpb.ConsumerInitRequest) (*consumerpb.ConsumerInitReply, error) {
	group, version, err := s.getTargetRouting(ctx, req.GetGroupName(), req.GetHostName())
	if err != nil {
		mlog.Errorf("failed to init consumer: %v", err)
		return nil, util.ErrorWithPos(err)
	}
	return &consumerpb.ConsumerInitReply{
		Group:   group,
		Version: version,
	}, nil
}

func (s *RoutingServer) getTargetRouting(ctx context.Context, groupName string, hosts []string) (*routingpb.Group, int64, error) {
	// recover
	defer func() {
		if err := recover(); err != nil {
			mlog.Warnf("recovered from panic: %v", err)
		}
	}()
	routingTable := s.routingTable.Load()
	version := s.logVersion.Load()
	group, ok := routingTable.GetGroups()[groupName]
	if !ok {
		return nil, 0, util.ErrorfWithPos("group [%v] not exists", groupName)
	}

	resp := &routingpb.Group{
		Name:  groupName,
		Hosts: make(map[string]*routingpb.Host, len(hosts)),
	}

	for _, hostName := range hosts {
		host, ok := group.Hosts[hostName]
		if !ok {
			mlog.Infof("host [%v/%v] not exists", groupName, hostName)
			continue
		}
		resp.Hosts[hostName] = proto.Clone(host).(*routingpb.Host)
	}
	return resp, version, nil
}

func (s *RoutingServer) ConsumerUpdate(ctx context.Context, req *consumerpb.ConsumerUpdateRequest) (*consumerpb.ConsumerUpdateReply, error) {
	dif := s.logVersion.Load() - req.GetVersion()
	// consumer版本号过低，直接发全量的新表
	if dif > 2 || req.GetVersion() == 0 {
		mlog.Info("consumer version too low, dispatch all routing table")
		group, version, err := s.getTargetRouting(ctx, req.GetGroupName(), req.GetHostName())
		if err != nil {
			mlog.Errorf("failed to return init routing table: %v", err)
			return nil, util.ErrorWithPos(err)
		}
		return &consumerpb.ConsumerUpdateReply{
			Version:  version,
			Outdated: true,
			Group:    group,
		}, nil
	} else if dif < 0 {
		mlog.Fatal("routing server version too low")
	}

	// resp := &consumerpb.ConsumerUpdateReply{}
	// s.mu.RLock()
	// for i := int64(0); i < dif; i++ {

	// }

	return nil, nil
}
