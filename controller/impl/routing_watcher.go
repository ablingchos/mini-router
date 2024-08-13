package controller

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/common"
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
	routingLogPrefix     = "/routing-log"
	TimeZone             = "Asia/Shanghai"
)

type RoutingWatcher struct {
	etcdClient   *clientv3.Client
	routingTable *common.RoutingTable
	mu           sync.RWMutex
	version      atomic.Int64 // routing table的version = log的version + 1
	logWriter    *LogWriter
	ctx          context.Context
	cancelFunc   context.CancelFunc
	loc          *time.Location
}

func NewRoutingWatcher() (*RoutingWatcher, error) {
	routingWatcher := &RoutingWatcher{
		routingTable: &common.RoutingTable{},
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdUri},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	loc, err := time.LoadLocation(TimeZone)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	routingWatcher.loc = loc
	routingWatcher.etcdClient = client
	routingWatcher.logWriter = NewLogWriter()
	ctx, cancel := context.WithCancel(context.Background())
	routingWatcher.ctx = ctx
	routingWatcher.cancelFunc = cancel
	return routingWatcher, nil
}

func (r *RoutingWatcher) Run() {
	// 在 initRoutingTable 之前调用
	if err := r.checkEtcdHealth(); err != nil {
		mlog.Fatalf("etcd health check failed: %v", err)
	}
	if err := r.initRoutingTable(); err != nil {
		mlog.Fatalf("failed to init routing table: %v", err)
	}
	mlog.Debugf("init routing table successfully")
	go r.watchLoop()
	go r.updateLoop()
	mlog.Info("routing watcher start to work")
}

func (r *RoutingWatcher) checkEtcdHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.etcdClient.Status(ctx, r.etcdClient.Endpoints()[0])
	if err != nil {
		return err
	}
	mlog.Debug("etcd health check successful")
	return nil
}

func (r *RoutingWatcher) Stop() {
	mlog.Info("received shutdown signal, start to stop")
	r.etcdClient.Close()
	r.cancelFunc()
}

func (r *RoutingWatcher) initRoutingTable() error {
	// 创建一个分布式锁会话
	session, err := concurrency.NewSession(r.etcdClient, concurrency.WithTTL(5))
	if err != nil {
		mlog.Warnf("failed to create session: %v", err)
		return util.ErrorWithPos(err)
	}
	// 创建一个分布式锁
	mutex := concurrency.NewMutex(session, routingTableMutexKey)
	// 获取锁
	if err := mutex.Lock(r.ctx); err != nil {
		return util.ErrorWithPos(err)
	}
	defer func() {
		if err := mutex.Unlock(r.ctx); err != nil {
			mlog.Errorf("failed to release lock: %v", err)
		}
		if err := session.Close(); err != nil {
			mlog.Errorf("failed to close session: %v", err)
		}
	}()

	// 使用事务初始化值
	txn := r.etcdClient.Txn(r.ctx)
	// 事务操作：如果键不存在，则初始化它
	txnResp, err := txn.If(
		clientv3.Compare(clientv3.Version(routingTableKey), "=", 0),
	).Then(
		clientv3.OpPut(routingTableKey, ``),
	).Else(
		clientv3.OpGet(routingTableKey),
	).Commit()
	if err != nil {
		return util.ErrorWithPos(err)
	}

	r.version.Store(1)
	if txnResp.Succeeded {
		mlog.Infof("created routing table key for the first time")
	} else {
		kv := txnResp.Responses[0].GetResponseRange().Kvs[0]
		value := kv.Value
		if len(value) == 0 {
			mlog.Warn("routing table key exists but is empty")
			r.routingTable = &common.RoutingTable{}
		} else {
			routingTable := &routingpb.RoutingTable{}
			if err := json.Unmarshal(kv.Value, routingTable); err != nil {
				mlog.Debugf("value: %v, version: %v", string(kv.Value), kv.Version)
				return util.ErrorWithPos(err)
			}
			r.routingTable = (*common.RoutingTable)(routingTable)
			r.version.Store(txnResp.Responses[0].GetResponseRange().Kvs[0].Version)
		}

		mlog.Info("init routing watcher", zap.Any("routing table", r.routingTable), zap.Any("version", r.version.Load()))
	}

	return nil
}

func (r *RoutingWatcher) watchLoop() {
	watchChan := r.etcdClient.Watch(r.ctx, routingHeartbeatKey, clientv3.WithPrefix())
	for watchResp := range watchChan {
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
					r.addEndpoint(groupName, hostName, endpoint)
					r.logWriter.write(groupName, hostName, endpoint, routingpb.ChangeType_add, r.loc)
				} else {
					r.updateEndpoint(groupName, hostName, endpoint)
					r.logWriter.write(groupName, hostName, endpoint, routingpb.ChangeType_update, r.loc)
				}
			case mvccpb.DELETE:
				r.deleteEndpoint(groupName, hostName, eidStr)
				r.logWriter.write(groupName, hostName, nil, routingpb.ChangeType_delete, r.loc)
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
				mlog.Warnf("failed to update routing to etcd: %v", err)
				break
			}
			// if err := r.reportChangeRecordsToEtcd(); err != nil {
			// 	mlog.Warnf("failed to report change log to etcd: %v", err)
			// 	break
			// }
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

	version := r.version.Load()
	txn := r.etcdClient.Txn(r.ctx)
	txnResp, err := txn.If(
		clientv3.Compare(clientv3.Version(routingTableKey), "<", version+1),
	).Then(
		clientv3.OpPut(routingTableKey, string(bytes)),
	).Else(
		// clientv3.OpGet(routingTableKey),
		clientv3.OpPut(routingTableKey, string(bytes)),
	).Commit()
	if err != nil {
		return util.ErrorWithPos(err)
	}

	if txnResp.Succeeded {
		r.version.Add(1)
		mlog.Info("update routing table to etcd successfully",
			zap.Any("routing table", routing), zap.Any("version", version))
	} else {
		mlog.Warn("routing table version higher than local version",
			zap.Any("remote version", txnResp.Responses[0].GetResponseRange().Kvs[0].Version), zap.Any("local version", version))
		return util.ErrorfWithPos("failed to update routing table: version mismatched")
	}
	return nil
}

func (r *RoutingWatcher) reportChangeRecordsToEtcd() error {
	// log version = routing table version - 1
	version := r.version.Load() - 1
	records := r.logWriter.flush(version)
	bytes, err := json.Marshal(records)
	if err != nil {
		return util.ErrorWithPos(err)
	}

	addKey := routingLogPrefix + "/" + strconv.Itoa(int(version))
	deleteKey := routingLogPrefix + "/" + strconv.Itoa(int(version-changeLogLength))
	// 事务操作
	txn := r.etcdClient.Txn(r.ctx)
	txnResp, err := txn.If(
		clientv3.Compare(clientv3.Version(addKey), "=", 0),
	).Then(
		clientv3.OpPut(addKey, string(bytes)),
		clientv3.OpDelete(deleteKey),
	).Else(
		clientv3.OpGet(addKey),
	).Commit()
	if err != nil {
		return util.ErrorWithPos(err)
	}

	if txnResp.Succeeded {
		mlog.Infof("report change log successfully", zap.Any("change log", records))
	} else {
		mlog.Error("change log exists in etcd", zap.Any(addKey, string(txnResp.Responses[0].GetResponseRange().Kvs[0].Value)))
		return util.ErrorfWithPos("failed to report change log, log already exists")
	}

	return nil
}

func (r *RoutingWatcher) addEndpoint(groupName string, hostName string, endpointInfo *routingpb.Endpoint) {
	// add endpoint
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.routingTable.Insert(groupName, hostName, endpointInfo); err != nil {
		mlog.Errorf("failed to add endpoint, err: %v", err)
	}
}

func parseHeartbeatKey(key string) (string, string, string, error) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != strings.TrimPrefix(routingHeartbeatKey, "/") {
		return "", "", "", util.ErrorfWithPos("wrong format to add an endpoint")
	}
	return words[1], words[2], words[3], nil
}

func (r *RoutingWatcher) updateEndpoint(groupName string, hostName string, endpointInfo *routingpb.Endpoint) {
	// // update endpoint
	r.mu.Lock()
	defer r.mu.Unlock()
	// err = s.routingTable.update(words[1], words[2], endpointInfo)
	// if err != nil {
	// 	mlog.Errorf("failed to update endpoint, err: %v", err)
	// }
}

func (r *RoutingWatcher) deleteEndpoint(groupName string, hostName string, eidStr string) {
	// delete key
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.routingTable.Delete(groupName, hostName, eidStr); err != nil {
		mlog.Errorf("failed to delete endpoint, err: %v", err)
	}
}

func (r *RoutingWatcher) cloneRoutingTable() *routingpb.RoutingTable {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cloned, _ := proto.Clone((*routingpb.RoutingTable)(r.routingTable)).(*routingpb.RoutingTable)
	return cloned
}
