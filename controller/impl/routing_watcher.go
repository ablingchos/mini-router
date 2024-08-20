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
					addedEndpoints[groupName+"/"+hostName+"/"+eidStr] = endpoint
					r.metrics.incrServerNumber()
				}
			case mvccpb.DELETE:
				deletedEndpoints[groupName+"/"+hostName+"/"+eidStr] = struct{}{}
				r.metrics.descServerNumber()
			default:
				mlog.Errorf("invalid type of etcd event: %v", event)
			}
		}
		// if r.getRedisLock() {
		r.updateRouting(addedEndpoints, deletedEndpoints)
		// }
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
	if len(addedEndpoints) == 0 {
		return
	}

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

	mlog.Debugf("insert endpoint: %v", grouped)
	for host, endpoints := range grouped {
		if err := r.redisClient.HSet(r.ctx, host, endpoints).Err(); err != nil {
			mlog.Warnf("failed to hset to redis: %v", err)
			continue
		}
	}
}

func (r *RoutingWatcher) processDelete(deletedEndpoints map[string]struct{}) {
	if len(deletedEndpoints) == 0 {
		return
	}

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
		resp := r.redisClient.HDel(r.ctx, host, endpoints...)
		if resp.Err() != nil {
			mlog.Warnf("failed to hdel to redis: %v", resp.Err())
			continue
		}
		mlog.Debugf("deleted keys: %v", resp.Val())
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
