package controller

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

const (
	updateInterval       = 2
	changeLogLength      = 3
	routingTableKey      = "/routing-table"
	routingTableMutexKey = "/mutex/routing-table"
	routingDiffPrefix    = "/routing-diff"
)

type RoutingWatcher struct {
	etcdClient   *clientv3.Client
	routingTable *RoutingTable
	mu           sync.RWMutex
	version      atomic.Int64
	changeLog    *ChangeRecords
	ctx          context.Context
}

func NewRoutingWatcher() (*RoutingWatcher, error) {
	routingWatcher := &RoutingWatcher{
		routingTable: &RoutingTable{
			Groups: make(map[string]*routingpb.Group),
		},
		ctx:       context.Background(),
		changeLog: &ChangeRecords{},
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdUri},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	routingWatcher.etcdClient = client
	return routingWatcher, nil
}

func (r *RoutingWatcher) Run() {
	if err := r.initRoutingTable(); err != nil {
		mlog.Fatalf("failed to init routing table: %v", err)
	}
	go r.watchLoop()
	go r.updateLoop()
	mlog.Info("routing watcher start to work")
}

func (r *RoutingWatcher) Stop() {
	mlog.Info("received shutdown signal, start to stop")
	r.ctx.Done()
}

func (r *RoutingWatcher) initRoutingTable() error {
	ctx := context.Background()
	// 创建一个分布式锁会话
	session, err := concurrency.NewSession(r.etcdClient)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	// 创建一个分布式锁
	mutex := concurrency.NewMutex(session, routingTableMutexKey)
	// 获取锁
	if err := mutex.Lock(ctx); err != nil {
		return util.ErrorWithPos(err)
	}
	defer func() {
		if err := mutex.Unlock(ctx); err != nil {
			mlog.Errorf("failed to release lock: %v", err)
		}
		if err := session.Close(); err != nil {
			mlog.Errorf("failed to close session: %v", err)
		}
	}()
	// 使用事务初始化值
	txn := r.etcdClient.Txn(ctx)
	// 事务操作：如果键不存在，则初始化它
	txnResp, err := txn.If(
		clientv3.Compare(clientv3.Version(routingTableKey), "=", 0),
	).Then(
		clientv3.OpPut(routingTableKey, ""),
	).Else(
		clientv3.OpGet(routingTableKey),
	).Commit()
	if err != nil {
		return util.ErrorWithPos(err)
	}

	if txnResp.Succeeded {
		r.version.Store(1)
		mlog.Infof("created routing table key for the first time")
	} else {
		routingTable := &routingpb.RoutingTable{}
		if err := json.Unmarshal(txnResp.Responses[0].GetResponseRange().Kvs[0].Value, routingTable); err != nil {
			return util.ErrorWithPos(err)
		}
		r.routingTable = (*RoutingTable)(routingTable)
		r.version.Store(txnResp.Responses[0].GetResponseRange().Kvs[0].Version)
		mlog.Info("init routing watcher", zap.Any("routing table", r.routingTable), zap.Any("version", r.version.Load()))
	}

	return nil
}

func (r *RoutingWatcher) watchLoop() {
	watchChan := r.etcdClient.Watch(r.ctx, routingHeartbeatKey, clientv3.WithPrefix())
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			key := strings.TrimPrefix(string(event.Kv.Key), "/")
			mlog.Debugf("Received etcd event: %v", event)
			switch event.Type {
			case mvccpb.PUT:
				value := &routingpb.Endpoint{}
				if err := json.Unmarshal(event.Kv.Value, value); err != nil {
					mlog.Errorf("failed to unmarshal value: %v", event.Kv.Value)
					break
				}
				if event.IsCreate() {
					r.addEndpoint(key, value)
				} else {
					r.updateEndpoint(key, value)
				}
			case mvccpb.DELETE:
				r.deleteEndpoint(key)
			default:
				mlog.Errorf("invalid type of etcd event: %v", event)
			}
		}
	}
}

func (r *RoutingWatcher) updateLoop() {
	ticker := time.NewTicker(updateInterval * time.Second)
	for {
		select {
		case <-ticker.C:
			if err := r.updateRoutingToEtcd(); err != nil {
				mlog.Errorf("failed to update routing to etcd: %v", err)
				break
			}

		case <-r.ctx.Done():
			mlog.Info("received shutdown signal, update stopped")
			return
		}
	}
}

func (r *RoutingWatcher) updateRoutingToEtcd() error {
	routing := r.cloneRoutingTable()
	bytes, err := json.Marshal(routing)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	ctx := context.Background()
	txn := r.etcdClient.Txn(ctx)
	txn.If(
		clientv3.Compare(clientv3.Version(routingTableKey), "<=", r.version.Load()),
	).Then(
		clientv3.OpPut(routingTableKey, string(bytes)),
	).Else(
		clientv3.OpGet(routingTableKey),
	)

	txnResp, err := txn.Commit()
	if err != nil {
		return util.ErrorWithPos(err)
	}

	if txnResp.Succeeded {
		r.version.Add(1)
		mlog.Info("update routing table to etcd successfully",
			zap.Any("routing table", routing), zap.Any("version", r.version.Load()))
	} else {
		mlog.Warn("routing table version higher than local version",
			zap.Any("remote version", txnResp.Responses[0].GetResponseRange().Kvs[0].Version), zap.Any("local version", r.version.Load()))
		return util.ErrorfWithPos("failed to update routing table: version mismatched")
	}
	return nil
}

func (r *RoutingWatcher) addEndpoint(key string, endpointInfo *routingpb.Endpoint) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != routingHeartbeatKey {
		mlog.Warn("wrong format to add an endpoint", zap.Any("key", key), zap.Any("value", endpointInfo))
		return
	}

	// add endpoint
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.routingTable.Insert(words[1], words[2], endpointInfo); err != nil {
		mlog.Errorf("failed to add endpoint, err: %v", err)
	}
}

func (r *RoutingWatcher) updateEndpoint(key string, endpointInfo *routingpb.Endpoint) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != routingHeartbeatKey {
		mlog.Warn("wrong format to add an endpoint", zap.Any("key", key), zap.Any("value", endpointInfo))
		return
	}

	// // update endpoint
	r.mu.Lock()
	defer r.mu.Unlock()
	// err = s.routingTable.update(words[1], words[2], endpointInfo)
	// if err != nil {
	// 	mlog.Errorf("failed to update endpoint, err: %v", err)
	// }
	mlog.Debugf("update endpoint [%v/%v/%v]", words[1], words[2], words[3])
}

func (r *RoutingWatcher) deleteEndpoint(key string) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != routingHeartbeatKey {
		mlog.Warn("wrong format to delete an endpoint", zap.Any("key", key))
		return
	}
	// delete key
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.routingTable.Delete(words[1], words[2], words[3]); err != nil {
		mlog.Errorf("failed to delete endpoint, err: %v", err)
	}
}

func (r *RoutingWatcher) cloneRoutingTable() *routingpb.RoutingTable {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cloned, _ := proto.Clone((*routingpb.RoutingTable)(r.routingTable)).(*routingpb.RoutingTable)
	return cloned
}
