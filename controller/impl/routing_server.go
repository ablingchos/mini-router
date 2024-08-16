package controller

import (
	"context"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	redis "github.com/go-redis/redis/v8"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	pullInterval      = 3
	routingServerPort = ":5200"
	cacheSize         = 100
)

type RoutingServer struct {
	etcdClient  *clientv3.Client
	redisClient *redis.Client
	// routingTable atomic.Pointer[routingpb.RoutingTable]
	routingCache []string
	cacheMap     map[string]*routingpb.Host
	ctx          context.Context
	cancelFunc   context.CancelFunc
	loc          *time.Location
	changeLogs   []*routingpb.ChangeRecords
	mu           sync.RWMutex
	metrics      *Metrics

	consumerpb.UnimplementedConsumerServiceServer
}

func NewRoutingServer() (*RoutingServer, error) {
	server := &RoutingServer{
		changeLogs: make([]*routingpb.ChangeRecords, changeLogLength, changeLogLength),
		// routingTable: atomic.Pointer[routingpb.RoutingTable]{},
		routingCache: make([]string, 0, cacheSize),
		cacheMap:     make(map[string]*routingpb.Host, cacheSize),
		metrics:      NewMetrics("routing_server"),
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdUri},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}

	redis := redis.NewClient(&redis.Options{
		Addr:     redisUri,
		Password: "",
		DB:       0,
	})

	server.redisClient = redis
	server.etcdClient = client
	ctx, cancel := context.WithCancel(context.Background())
	server.ctx = ctx
	server.cancelFunc = cancel
	return server, nil
}

func (s *RoutingServer) Run() {
	go s.metrics.Start("localhost" + routingServerPort)
	go s.serveGrpc()
	go s.watchLoop()
	// go s.pullLoop()
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

func (s *RoutingServer) getRedis(groupName string, hostName string) (*routingpb.Host, error) {
	hostKey := groupName + "/" + hostName
	resp, err := s.redisClient.HGetAll(s.ctx, hostKey).Result()
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	host := &routingpb.Host{
		Name:      hostName,
		Endpoints: make(map[int64]*routingpb.Endpoint, len(resp)),
	}
	for eidStr, value := range resp {
		eid, err := strconv.Atoi(eidStr)
		if err != nil {
			mlog.Errorf("failed to convert string to int: %v, str: %v", err, eidStr)
			continue
		}

		endpoint := &routingpb.Endpoint{}
		if err := json.Unmarshal([]byte(value), endpoint); err != nil {
			mlog.Warnf("failed to unmarshal: %v", value)
		}
		host.Endpoints[int64(eid)] = endpoint
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.routingCache) >= cacheSize {
		deleteKey := s.routingCache[0]
		s.routingCache = s.routingCache[1:]
		delete(s.cacheMap, deleteKey)
	}
	s.routingCache = append(s.routingCache, hostKey)
	s.cacheMap[hostKey] = host
	return host, nil
}

func (s *RoutingServer) watchLoop() {
	watchChan := s.etcdClient.Watch(s.ctx, routingHeartbeatKey, clientv3.WithPrefix())
	for watchResp := range watchChan {
		addedEndpoints := make(map[string]*routingpb.Endpoint, 0)
		deletedEndpoints := make(map[string]struct{}, 0)
		for _, event := range watchResp.Events {
			key := strings.TrimPrefix(string(event.Kv.Key), "/")
			groupName, hostName, eidStr, err := parseHeartbeatKey(key)
			if err != nil {
				mlog.Warn("wrong heartbeat key format", zap.Any("key", key))
				continue
			}
			mlog.Debugf("Received etcd event: %v", event)
			switch event.Type {
			case mvccpb.PUT:
				endpoint := &routingpb.Endpoint{}
				if err := json.Unmarshal(event.Kv.Value, endpoint); err != nil {
					mlog.Errorf("failed to unmarshal endpoint: %v", event.Kv.Value)
					break
				}
				if event.IsCreate() {
					addedEndpoints[groupName+"/"+hostName+"/"+eidStr] = endpoint
					s.addLocalCache(groupName+"/"+hostName, endpoint.GetEid(), endpoint)
				}
			case mvccpb.DELETE:
				eid, _ := strconv.Atoi(eidStr)
				deletedEndpoints[groupName+"/"+hostName+"/"+eidStr] = struct{}{}
				s.deleteLocalCache(groupName+"/"+hostName, int64(eid))
			default:
				mlog.Errorf("invalid type of etcd event: %v", event)
			}
		}
	}

}

func (s *RoutingServer) deleteLocalCache(hostKey string, eid int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cacheMap[hostKey]; ok {
		delete(s.cacheMap[hostKey].GetEndpoints(), eid)
	}
}

func (s *RoutingServer) addLocalCache(hostKey string, eid int64, endpoint *routingpb.Endpoint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cacheMap[hostKey]; ok {
		s.cacheMap[hostKey].Endpoints[eid] = endpoint
		return
	}
}

// func (s *RoutingServer) pullLoop() {
// 	ticker := time.NewTicker(pullInterval * time.Second)
// 	for {
// 		select {
// 		case <-ticker.C:
// 			if err := s.updateLocalRoutingTable(); err != nil {
// 				mlog.Errorf("failed to update local routing table: %v", err)
// 			}
// 		case <-s.ctx.Done():
// 			mlog.Info("pull loop stopped")
// 			return
// 		}
// 	}
// }

// func (s *RoutingServer) updateLocalRoutingTable() error {
// 	resp, err := s.etcdClient.Get(s.ctx, routingTableKey)
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}
// 	routingTable := &routingpb.RoutingTable{}
// 	if len(resp.Kvs) == 0 {
// 		return util.ErrorfWithPos("routing table on etcd is nil")
// 	}
// 	if err := json.Unmarshal(resp.Kvs[0].Value, routingTable); err != nil {
// 		return util.ErrorWithPos(err)
// 	}
// 	s.routingTable.Store(routingTable)
// 	mlog.Debug("update routing table", zap.Any("routing table", routingTable))
// 	return nil
// }

// func (s *RoutingServer) watchLoop() {
// 	watchChan := s.etcdClient.Watch(s.ctx, routingLogPrefix, clientv3.WithPrefix())
// 	for watchResp := range watchChan {
// 		for _, event := range watchResp.Events {
// 			mlog.Debugf("Received etcd event: %v", event)
// 			switch event.Type {
// 			case mvccpb.PUT:
// 				log := &routingpb.ChangeRecords{}
// 				if err := json.Unmarshal(event.Kv.Value, log); err != nil {
// 					mlog.Errorf("failed to unmarshal change log: %v", err)
// 					break
// 				}
// 				if !event.IsCreate() {
// 					mlog.Warn("change log put again", zap.Any("change log", log))
// 					break
// 				}
// 				s.updateLog(log)
// 			}
// 		}
// 	}
// }

// func (s *RoutingServer) updateLog(log *routingpb.ChangeRecords) {
// 	targetVersion := log.GetVersion()
// 	// 版本号初始化为最新version - 1
// 	if s.logVersion.Load() == 0 {
// 		s.logVersion.Store(targetVersion - 1)
// 	}
// 	currentVersion := s.logVersion.Load()
// 	if currentVersion >= targetVersion {
// 		mlog.Fatal("log version illegal", zap.Any("current log version", currentVersion), zap.Any("target version", targetVersion))
// 	} else if currentVersion+3 < targetVersion {
// 		s.logVersion.Store(targetVersion)
// 		s.clearLog()
// 		mlog.Info("log version too low, clear all log")
// 	} else {
// 		s.logVersion.Add(1)
// 	}
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	// 队头出队
// 	s.changeLogs = s.changeLogs[1:]
// 	// 队尾入队
// 	s.changeLogs = append(s.changeLogs, log)
// 	mlog.Info("updated log", zap.Any("log version", targetVersion), zap.Any("log", log))
// }

// func (s *RoutingServer) clearLog() {
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	s.changeLogs = make([]*routingpb.ChangeRecords, changeLogLength, changeLogLength)
// }

func (s *RoutingServer) ConsumerInit(ctx context.Context, req *consumerpb.ConsumerInitRequest) (*consumerpb.ConsumerInitReply, error) {
	group, err := s.getTargetRouting(ctx, req.GetGroupName(), req.GetHostName())
	if err != nil {
		mlog.Errorf("failed to init consumer: %v", err)
		return nil, util.ErrorWithPos(err)
	}
	return &consumerpb.ConsumerInitReply{
		Group: group,
	}, nil
}

func (s *RoutingServer) getHost(groupName string, hostName string) (*routingpb.Host, error) {
	s.mu.RLock()
	if _, ok := s.cacheMap[groupName+"/"+hostName]; ok {
		s.mu.RUnlock()
		return s.cacheMap[groupName+"/"+hostName], nil
	}

	s.mu.RUnlock()
	host, err := s.getRedis(groupName, hostName)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	return host, err
}

func (s *RoutingServer) getTargetRouting(ctx context.Context, groupName string, hosts []string) (*routingpb.Group, error) {
	// recover
	group := &routingpb.Group{
		Name:  groupName,
		Hosts: make(map[string]*routingpb.Host, len(hosts)),
	}
	for _, hostName := range hosts {
		host, err := s.getHost(groupName, hostName)
		if err != nil {
			mlog.Warnf("failed to get host [%v %v]", groupName, hostName)
			continue
		}
		group.Hosts[hostName] = host
	}
	return group, nil
}

// func (s *RoutingServer) ConsumerUpdate(ctx context.Context, req *consumerpb.ConsumerUpdateRequest) (*consumerpb.ConsumerUpdateReply, error) {
// 	// consumer版本号过低，直接发全量的新表
// 	if dif > 2 || req.GetVersion() == 0 {
// 		mlog.Info("consumer version too low, dispatch all routing table")
// 		group, version, err := s.getTargetRouting(ctx, req.GetGroupName(), req.GetHostName())
// 		if err != nil {
// 			mlog.Errorf("failed to return init routing table: %v", err)
// 			return nil, util.ErrorWithPos(err)
// 		}
// 		return &consumerpb.ConsumerUpdateReply{
// 			Version:  version,
// 			Outdated: true,
// 			Group:    group,
// 		}, nil
// 	} else if dif < 0 {
// 		mlog.Fatal("routing server version too low")
// 	}

// 	// resp := &consumerpb.ConsumerUpdateReply{}
// 	// s.mu.RLock()
// 	// for i := int64(0); i < dif; i++ {

// 	// }

// 	return nil, nil
// }
