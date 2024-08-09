package controller

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"git.woa.com/kefuai/mini-router/internal/common"
	"git.woa.com/kefuai/mini-router/pkg/proto/consumerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

const (
	pullInterval = 5
)

type DiscoverServer struct {
	etcdClient   *clientv3.Client
	routingTable *common.RoutingTable
	changeLogs   []*routingpb.ChangeRecords
	mu           sync.RWMutex
	version      atomic.Int64
	ctx          context.Context
	cancelFunc   context.CancelFunc
	loc          *time.Location

	consumerpb.UnimplementedConsumerServiceServer
}

func NewRoutingServer() (*DiscoverServer, error) {
	server := &DiscoverServer{
		changeLogs: make([]*routingpb.ChangeRecords, changeLogLength, changeLogLength),
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

func (s *DiscoverServer) Run() {
	go s.watchLoop()
	go s.pullLoop()
}

func (s *DiscoverServer) Stop() {
	s.cancelFunc()
}

func (s *DiscoverServer) pullLoop() {
	ticker := time.NewTicker(pullInterval * time.Second)
	for {
		select {
		case <-ticker.C:

		case <-s.ctx.Done():
			mlog.Info("pull loop stopped")
			return
		}
	}
}

func (s *DiscoverServer) watchLoop() {
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

func (s *DiscoverServer) updateLog(log *routingpb.ChangeRecords) {
	currentVersion := s.version.Load()
	targetVersion := log.GetVersion()
	if currentVersion >= targetVersion || currentVersion+3 < targetVersion {
		mlog.Fatal("log version mismatched", zap.Any("current log version", currentVersion), zap.Any("target version", targetVersion))
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// 对头出队
	s.changeLogs = s.changeLogs[1:]
	// 队尾入队
	s.changeLogs = append(s.changeLogs, log)
	mlog.Info("updated log", zap.Any("log version", targetVersion), zap.Any("log", log))
}

func (s *DiscoverServer) ConsumerInit(context.Context, *consumerpb.ConsumerInitRequest) (*consumerpb.ConsumerInitReply, error) {
	resp, err := s.etcdClient.Get(s.ctx, routingTableKey)
	if err != nil {
		mlog.Warnf("failed to get routing table from etcd: %v", err)
		return nil, util.ErrorWithPos(err)
	}

	return nil, nil
}
func (s *DiscoverServer) ConsumerUpdate(context.Context, *consumerpb.ConsumerUpdateRequest) (*consumerpb.ConsumerUpdateReply, error) {
	return nil, nil
}
