package controller

import (
	"context"
	"net/http"
	"sync"
	"time"

	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"github.com/samber/lo"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	// for etcd
	httpGatewayKey  = "/controller/httpserver/addr"
	serverKeyPrefix = "/controller/server/addr"
	grpcPort        = "6000"
	httpPort        = "28080"
	// for http
	serverRegisterPath     = "/controller/server/register"
	providerRegisterPath   = "/provider/register"
	providerHeartbeatPath  = "/provider/heartbeat"
	providerUnregisterPath = "/provider/unregister"
	consumerInitPath       = "/consumer/init"
	consumerUpdatePath     = "/consumer/update"
)

var (
	httpGateway *HttpGateway
	once        sync.Once
)

type HttpGateway struct {
	etcdClient      *clientv3.Client
	addr            *IpPort
	serverAddrs     []string
	mu              sync.RWMutex
	lb              *Balancer
	watchCancelFunc context.CancelFunc
}

type IpPort struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

func (a *IpPort) getAddrString() string {
	return a.IP + ":" + a.Port
}

func GetHttpGateway() (*HttpGateway, error) {
	var err error
	once.Do(func() {
		httpGateway := &HttpGateway{
			addr:        &IpPort{},
			serverAddrs: make([]string, 0),
			lb:          &Balancer{},
		}
		httpGateway.etcdClient, err = clientv3.New(clientv3.Config{
			Endpoints:   []string{etcdUri},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return
		}
		httpGateway.addr.IP, _ = getIpAddr()
		httpGateway.addr.Port = grpcPort
		httpGateway.etcdClient.Put(context.Background(), httpGatewayKey, httpGateway.addr.getAddrString())
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	return httpGateway, nil
}

func (h *HttpGateway) Run() {
	go h.serveHttp()
}

func (h *HttpGateway) Stop() {
	h.watchCancelFunc()
}

func (h *HttpGateway) watchServer(key string) {
	// 校验key是否在etcd中已经注册
	time.Sleep(3 * time.Second)
	kvs, err := h.etcdClient.Get(context.Background(), key)
	if err != nil {
		mlog.Warnf("failed to get key: %v, err: %v", key, err)
		return
	}
	if len(kvs.Kvs) < 1 {
		mlog.Warnf("key not found: %v", key)
		return
	}
	// 开始watch租约
	ctx, cancel := context.WithCancel(context.Background())
	h.watchCancelFunc = cancel
	watchCh := h.etcdClient.Watch(ctx, key)
	for watchResp := range watchCh {
		for _, event := range watchResp.Events {
			switch event.Type {
			case mvccpb.DELETE:
				h.mu.Lock()
				lo.DropWhile(h.serverAddrs, func(item string) bool {
					return item == key
				})
				h.mu.Unlock()
				mlog.Infof("key expired, stop to watch: %v", key)
				return
			case mvccpb.PUT:
				mlog.Infof("key heartbeat: %v", key)
			default:
				mlog.Errorf("invalid type of etcd event: %v", event)
			}
		}
	}
}

func (h *HttpGateway) serveHttp() {
	mux := http.NewServeMux()
	mux.Handle("/", &httpHandler{})
	middlewareMux := httpMiddleware(mux)
	mlog.Info("start to serve http")

	if err := http.ListenAndServe(":"+httpPort, middlewareMux); err != nil {
		mlog.Fatalf("http error: %v", err)
	}
}
