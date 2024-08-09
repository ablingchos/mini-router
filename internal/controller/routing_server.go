package controller

import (
	"context"
	"encoding/json"
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
)

type DiscoverServer struct {
	etcdClient *clientv3.Client
	changeLogs []*routingpb.ChangeRecords
	mu         sync.RWMutex
	version    atomic.Int64
	ctx        context.Context

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
	return server, nil
}

func (s *DiscoverServer) Run() {

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

			}
		}
	}
}

func (s *DiscoverServer) formatLog(originLog *routingpb.ChangeRecords) error {
	patch := RoutingTable{
		Groups: make(map[string]*routingpb.Group),
	}
	currentVersion := s.version.Load()
	targetVersion := originLog.GetVersion()
	if currentVersion >= targetVersion || currentVersion+3 < targetVersion {
		mlog.Fatal("log version mismatched", zap.Any("current log version", currentVersion), zap.Any("target version", targetVersion))
	}

	return nil
}
