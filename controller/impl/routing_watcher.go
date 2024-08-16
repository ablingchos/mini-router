package controller

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/common"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	redis "github.com/go-redis/redis/v8"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
)

const (
	updateInterval    = 2
	changeLogLength   = 3
	routingTableKey   = "/routing"
	routingTableMutex = "/mutexs"
	routingLogPrefix  = "/routing-log"
	TimeZone          = "Asia/Shanghai"
	redisUri          = "localhost:6379"
)

type RoutingWatcher struct {
	etcdClient   *clientv3.Client
	redisClient  *redis.Client
	routingTable *common.RoutingTable
	mu           sync.RWMutex
	// version      atomic.Int64 // routing table的version = log的version + 1
	logWriter  *LogWriter
	ctx        context.Context
	cancelFunc context.CancelFunc
	loc        *time.Location
	metrics    *Metrics
}

func NewRoutingWatcher() (*RoutingWatcher, error) {
	routingWatcher := &RoutingWatcher{
		routingTable: &common.RoutingTable{},
		metrics:      NewMetrics("routing_watcher"),
	}
	etcd, err := clientv3.New(clientv3.Config{
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

	redis := redis.NewClient(&redis.Options{
		Addr:     redisUri,
		Password: "",
		DB:       0,
	})

	routingWatcher.redisClient = redis
	routingWatcher.loc = loc
	routingWatcher.etcdClient = etcd
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
	mlog.Debugf("init routing table successfully")
	go r.metrics.Start("localhost:5300")
	go r.watchLoop()
	// go r.updateLoop()
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

// func (r *RoutingWatcher) initRoutingTable() error {
// 	// 创建一个分布式锁会话
// 	session, err := concurrency.NewSession(r.etcdClient, concurrency.WithTTL(5))
// 	if err != nil {
// 		mlog.Warnf("failed to create session: %v", err)
// 		return util.ErrorWithPos(err)
// 	}

// 	defer session.Close()

// 	resp, err := r.etcdClient.Get(r.ctx, routingTableKey, clientv3.WithPrefix())
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}

// 	r.mu.Lock()
// 	defer r.mu.Unlock()

// 	for _, kv := range resp.Kvs {
// 		key := string(kv.Key)
// 		bytes := kv.Value
// 		divide := strings.Split(strings.TrimPrefix(key, "/"), "/")
// 		if len(divide) != 3 {
// 			mlog.Errorf("wrong routing host length: %v", key)
// 			continue
// 		}
// 		groupName, hostName := divide[1], divide[2]
// 		host := &routingpb.Host{}
// 		if err := json.Unmarshal(bytes, host); err != nil {
// 			return util.ErrorfWithPos("failed to unmarshal: %v", err)
// 		}
// 		r.addHost(groupName, hostName, host)
// 	}

// 	// r.version.Store(1)
// 	// if txnResp.Succeeded {
// 	// 	mlog.Infof("created routing table key for the first time")
// 	// } else {
// 	// 	kv := txnResp.Responses[0].GetResponseRange().Kvs[0]
// 	// 	value := kv.Value
// 	// 	if len(value) == 0 {
// 	// 		mlog.Warn("routing table key exists but is empty")
// 	// 		r.routingTable = &common.RoutingTable{}
// 	// 	} else {
// 	// 		routingTable := &routingpb.RoutingTable{}
// 	// 		if err := json.Unmarshal(kv.Value, routingTable); err != nil {
// 	// 			mlog.Debugf("value: %v, version: %v", string(kv.Value), kv.Version)
// 	// 			return util.ErrorWithPos(err)
// 	// 		}
// 	// 		r.routingTable = (*common.RoutingTable)(routingTable)
// 	// 		r.version.Store(txnResp.Responses[0].GetResponseRange().Kvs[0].Version)
// 	// 	}

// 	// 	mlog.Info("init routing watcher", zap.Any("routing table", r.routingTable), zap.Any("version", r.version.Load()))
// 	// }

// 	return nil
// }

func (r *RoutingWatcher) watchLoop() {
	watchChan := r.etcdClient.Watch(r.ctx, routingHeartbeatKey, clientv3.WithPrefix())
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
					// r.addEndpoint(groupName, hostName, endpoint)
					addedEndpoints[groupName+"/"+hostName+"/"+eidStr] = endpoint
					// r.logWriter.write(groupName, hostName, endpoint, routingpb.ChangeType_add, r.loc)
				} else {
					// r.updateEndpoint(groupName, hostName, endpoint)
					// r.logWriter.write(groupName, hostName, endpoint, routingpb.ChangeType_update, r.loc)
				}
			case mvccpb.DELETE:
				// r.deleteEndpoint(groupName, hostName, eidStr)
				deletedEndpoints[groupName+"/"+hostName+"/"+eidStr] = struct{}{}
				// r.logWriter.write(groupName, hostName, nil, routingpb.ChangeType_delete, r.loc)
			default:
				mlog.Errorf("invalid type of etcd event: %v", event)
			}
		}
		if r.getRedisLock() {
			r.updateRouting(addedEndpoints, deletedEndpoints)
		}
	}
}

func (r *RoutingWatcher) getRedisLock() bool {
	// 创建一个分布式锁会话
	session, err := concurrency.NewSession(r.etcdClient)
	if err != nil {
		return false
	}
	// 向etcd创建lease
	leaseResp, err := r.etcdClient.Grant(r.ctx, 5)
	if err != nil {
		return false
	}

	defer func() {
		if err := session.Close(); err != nil {
			mlog.Errorf("failed to close session: %v", err)
		}
	}()
	// 使用事务初始化值
	txn := r.etcdClient.Txn(r.ctx)
	// 事务操作：如果键不存在，则初始化它
	txnResp, err := txn.If(
		clientv3.Compare(clientv3.Version(routingTableMutex), "=", 0),
	).Then(
		clientv3.OpPut(routingTableMutex, "", clientv3.WithLease(leaseResp.ID)),
	).Commit()
	if err != nil {
		mlog.Errorf("failed to commit txn: %v", err)
		return false
	}

	if txnResp.Succeeded {
		return true
	}
	r.etcdClient.Delete(r.ctx, routingTableMutex)
	return false
}

func (r *RoutingWatcher) updateRouting(addedEndpoints map[string]*routingpb.Endpoint, deletedEndpoints map[string]struct{}) {
	r.processAdd(addedEndpoints)
	r.processDelete(deletedEndpoints)
}

func (r *RoutingWatcher) processAdd(addedEndpoints map[string]*routingpb.Endpoint) {
	grouped := make(map[string]map[string]string)

	for key, endpoint := range addedEndpoints {
		key = strings.TrimPrefix(key, "/")
		parts := strings.Split(key, "/")
		if len(parts) != 3 {
			mlog.Panicf("wrong format of key: %v", key)
		}
		bytes, err := json.Marshal(endpoint)
		if err != nil {
			mlog.Errorf("failed to marshal: %v", err)
			continue
		}
		newKey := parts[0] + "/" + parts[1]
		subKey := parts[2]
		if _, ok := grouped[newKey]; !ok {
			grouped[newKey] = make(map[string]string)
		}
		grouped[newKey][subKey] = string(bytes)
	}

	for host, endpoints := range grouped {
		if err := r.redisClient.HSet(r.ctx, host, endpoints).Err(); err != nil {
			mlog.Warnf("failed to hset to redis: %v", err)
			continue
		}
	}
}

func (r *RoutingWatcher) processDelete(deletedEndpoints map[string]struct{}) {
	grouped := make(map[string][]string)
	for key := range deletedEndpoints {
		key = strings.TrimPrefix(key, "/")
		parts := strings.Split(key, "/")
		if len(parts) != 3 {
			mlog.Panicf("wrong format of key: %v", key)
		}
		newKey := parts[0] + "/" + parts[1]
		subKey := parts[2]
		grouped[newKey] = append(grouped[newKey], subKey)
	}

	for host, endpoints := range grouped {
		if err := r.redisClient.HDel(r.ctx, host, endpoints...).Err(); err != nil {
			mlog.Warnf("failed to hdel to redis: %v", err)
			continue
		}
	}
}

func (r *RoutingWatcher) updateRoutingToEtcd(changedHosts map[string]struct{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	routing := r.routingTable

	for name := range changedHosts {
		words := strings.Split(name, "/")
		if len(words) != 2 {
			mlog.Warnf("wrong length of key: %v", name)
			continue
		}
		groupName, hostName := words[0], words[1]
		if _, ok := routing.Groups[groupName]; !ok {
			mlog.Warnf("no such group: %v", groupName)
		}
		r.updateEtcd(routingTableKey+"/"+name, r.routingTable.Groups[groupName].GetHosts()[hostName])

	}
}

func (r *RoutingWatcher) updateEtcd(key string, host *routingpb.Host) {

}

func (r *RoutingWatcher) addHost(groupName string, hostName string, host *routingpb.Host) {
	routing := r.routingTable
	if routing.Groups == nil {
		routing.Groups = make(map[string]*routingpb.Group)
	}
	if _, ok := routing.Groups[groupName]; !ok {
		routing.Groups[groupName] = &routingpb.Group{
			Name:  groupName,
			Hosts: make(map[string]*routingpb.Host),
		}
	}
	group := routing.Groups[groupName]

	if _, ok := group.GetHosts()[hostName]; ok {
		mlog.Warn("host already exists, skip inserting")
		return
	}

	group.GetHosts()[hostName] = host
}

func (r *RoutingWatcher) deleteHost(groupName string, hostName string, host *routingpb.Host) {
	routing := r.routingTable
	if routing.Groups == nil {
		mlog.Warn("routing table not exists, skip deleting")
		return
	}
	if _, ok := routing.Groups[groupName]; !ok {
		mlog.Warn("group not exists, skip deleting")
		return
	}
	group := routing.Groups[groupName]

	if _, ok := group.GetHosts()[hostName]; !ok {
		mlog.Warn("host not exists, skip deleting")
		return
	}

	delete(group.GetHosts(), hostName)
}

func (r *RoutingWatcher) addEndpoint(groupName string, hostName string, endpointInfo *routingpb.Endpoint) {
	// add endpoint
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.routingTable.Insert(groupName, hostName, endpointInfo); err != nil {
		mlog.Errorf("failed to add endpoint, err: %v", err)
	}
	r.metrics.incrServerNumber()
}

func (r *RoutingWatcher) deleteEndpoint(groupName string, hostName string, eidStr string) {
	// delete key
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.routingTable.Delete(groupName, hostName, eidStr); err != nil {
		mlog.Errorf("failed to delete endpoint, err: %v", err)
	}
	r.metrics.descServerNumber()
}

// func (r *RoutingWatcher) updateLoop() {
// 	ticker := time.NewTicker(updateInterval * time.Second)
// 	for {
// 		select {
// 		case <-ticker.C:
// 			if err := r.updateRoutingToEtcd(); err != nil {
// 				mlog.Warnf("failed to update routing to etcd: %v", err)
// 				break
// 			}
// 			// if err := r.reportChangeRecordsToEtcd(); err != nil {
// 			// 	mlog.Warnf("failed to report change log to etcd: %v", err)
// 			// 	break
// 			// }
// 		case <-r.ctx.Done():
// 			mlog.Info("received shutdown signal, update stopped")
// 			return
// 		}
// 	}
// }

// func (r *RoutingWatcher) updateRoutingToEtcd() error {
// 	routing := r.cloneRoutingTable()
// 	bytes, err := json.Marshal(routing)
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}

// 	version := r.version.Load()
// 	txn := r.etcdClient.Txn(r.ctx)
// 	txnResp, err := txn.If(
// 		clientv3.Compare(clientv3.Version(routingTableKey), "<", version+1),
// 	).Then(
// 		clientv3.OpPut(routingTableKey, string(bytes)),
// 	).Else(
// 		// clientv3.OpGet(routingTableKey),
// 		clientv3.OpPut(routingTableKey, string(bytes)),
// 	).Commit()
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}

// 	if txnResp.Succeeded {
// 		r.version.Add(1)
// 		r.metrics.setRoutingTableSize(int64(len(bytes)))
// 		mlog.Info("update routing table to etcd successfully",
// 			zap.Any("routing table", routing), zap.Any("version", version))
// 	} else {
// 		mlog.Warn("routing table version higher than local version",
// 			zap.Any("remote version", txnResp.Responses[0].GetResponseRange().Kvs[0].Version), zap.Any("local version", version))
// 		return util.ErrorfWithPos("failed to update routing table: version mismatched")
// 	}
// 	return nil
// }

// func (r *RoutingWatcher) reportChangeRecordsToEtcd() error {
// 	// log version = routing table version - 1
// 	version := r.version.Load() - 1
// 	records := r.logWriter.flush(version)
// 	bytes, err := json.Marshal(records)
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}

// 	addKey := routingLogPrefix + "/" + strconv.Itoa(int(version))
// 	deleteKey := routingLogPrefix + "/" + strconv.Itoa(int(version-changeLogLength))
// 	// 事务操作
// 	txn := r.etcdClient.Txn(r.ctx)
// 	txnResp, err := txn.If(
// 		clientv3.Compare(clientv3.Version(addKey), "=", 0),
// 	).Then(
// 		clientv3.OpPut(addKey, string(bytes)),
// 		clientv3.OpDelete(deleteKey),
// 	).Else(
// 		clientv3.OpGet(addKey),
// 	).Commit()
// 	if err != nil {
// 		return util.ErrorWithPos(err)
// 	}

// 	if txnResp.Succeeded {
// 		mlog.Infof("report change log successfully", zap.Any("change log", records))
// 	} else {
// 		mlog.Error("change log exists in etcd", zap.Any(addKey, string(txnResp.Responses[0].GetResponseRange().Kvs[0].Value)))
// 		return util.ErrorfWithPos("failed to report change log, log already exists")
// 	}

// 	return nil
// }

// func (r *RoutingWatcher) addEndpoint(groupName string, hostName string, endpointInfo *routingpb.Endpoint) {
// 	// add endpoint
// 	r.mu.Lock()
// 	defer r.mu.Unlock()

// 	if err := r.routingTable.Insert(groupName, hostName, endpointInfo); err != nil {
// 		mlog.Errorf("failed to add endpoint, err: %v", err)
// 	}
// 	r.metrics.incrQuestNumber()
// }

func parseHeartbeatKey(key string) (string, string, string, error) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != strings.TrimPrefix(routingHeartbeatKey, "/") {
		return "", "", "", util.ErrorfWithPos("wrong format to add an endpoint")
	}
	return words[1], words[2], words[3], nil
}

// func (r *RoutingWatcher) updateEndpoint(groupName string, hostName string, endpointInfo *routingpb.Endpoint) {
// 	// // update endpoint
// 	r.mu.Lock()
// 	defer r.mu.Unlock()
// 	// err = s.routingTable.update(words[1], words[2], endpointInfo)
// 	// if err != nil {
// 	// 	mlog.Errorf("failed to update endpoint, err: %v", err)
// 	// }
// }

// func (r *RoutingWatcher) deleteEndpoint(groupName string, hostName string, eidStr string) {
// 	// delete key
// 	r.mu.Lock()
// 	defer r.mu.Unlock()

// 	if err := r.routingTable.Delete(groupName, hostName, eidStr); err != nil {
// 		mlog.Errorf("failed to delete endpoint, err: %v", err)
// 	}
// 	r.metrics.descServerNumber()
// }

// func (r *RoutingWatcher) cloneRoutingTable() *routingpb.RoutingTable {
// 	r.mu.RLock()
// 	defer r.mu.RUnlock()
// 	cloned, _ := proto.Clone((*routingpb.RoutingTable)(r.routingTable)).(*routingpb.RoutingTable)
// 	return cloned
// }
