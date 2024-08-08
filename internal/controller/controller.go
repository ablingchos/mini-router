package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/providerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	Location *time.Location
)

const (
	routingTablePrefix = "routing"
	timeZone           = "Asia/Shanghai"
	snapshotInterval   = 2
)

type Server struct {
	etcdClient      *clientv3.Client
	leaseCancelFunc context.CancelFunc
	watchCancelFunc context.CancelFunc
	stopCh          chan struct{}
	addr            *IpPort
	routingTable    *RoutingTable
	mu              sync.RWMutex
	snapshot        atomic.Pointer[*routingpb.RoutingTable]
	lastUpdateTime  time.Time
}

func NewServer(port string) (*Server, error) {
	loc, err := time.LoadLocation(timeZone)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	Location = loc
	server := &Server{
		addr:   &IpPort{},
		stopCh: make(chan struct{}),
		routingTable: &RoutingTable{
			Groups: make(map[string]*routingpb.Group),
		},
		snapshot: atomic.Pointer[*routingpb.RoutingTable]{},
	}
	server.etcdClient, err = clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdUri},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	server.addr.IP, _ = getIpAddr()
	server.addr.Port = port

	return server, nil
}

func (s *Server) GetEtcdClient() *clientv3.Client {
	if s == nil {
		return nil
	}
	return s.etcdClient
}

func (s *Server) Start() error {
	err := s.initRoutingTable()
	if err != nil {
		return util.ErrorWithPos(err)
	}

	err = s.registerServer()
	if err != nil {
		return util.ErrorWithPos(err)
	}
	go s.watchLoop()
	return err
}

func (s *Server) Stop() {
	s.leaseCancelFunc()
	s.watchCancelFunc()
}

// 初始化server， 拉取全量路由表
func (s *Server) initRoutingTable() error {
	resp, err := s.etcdClient.Get(context.Background(), "/"+routingTablePrefix, clientv3.WithPrefix())
	if err != nil {
		return util.ErrorWithPos(err)
	}
	for _, kv := range resp.Kvs {
		key := strings.TrimPrefix(string(kv.Key), "/")
		value := &routingpb.Endpoint{}
		err := json.Unmarshal(kv.Value, value)
		if err != nil {
			mlog.Errorf("failed to unmarshal value: %v", kv.Value)
			continue
		}
		s.addEndpoint(key, value)
	}
	go s.snapshotLoop()
	return nil
}

func (s *Server) watchLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	s.watchCancelFunc = cancel
	watchChan := s.GetEtcdClient().Watch(ctx, "/"+routingTablePrefix, clientv3.WithPrefix())
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			key := strings.TrimPrefix(string(event.Kv.Key), "/")
			switch event.Type {
			case mvccpb.PUT:
				value := &routingpb.Endpoint{}
				err := json.Unmarshal(event.Kv.Value, value)
				if err != nil {
					mlog.Errorf("failed to unmarshal value: %v", event.Kv.Value)
					break
				}
				if event.IsCreate() {
					s.addEndpoint(key, value)
				} else {
					s.updateEndpoint(key, value)
				}
			case mvccpb.DELETE:
				s.deleteEndpoint(key)
			default:
				mlog.Errorf("invalid type of etcd event: %v", event)
			}
		}
	}
}

func (s *Server) snapshotLoop() {
	ticker := time.NewTicker(snapshotInterval * time.Second)
	for {
		select {
		case <-ticker.C:
			s.storeSnapshot()
		case <-s.stopCh:
			mlog.Info("receive stop signal, build tag has stopped")
			return
		}
	}
}

func (s *Server) storeSnapshot() {
	snapshot := s.cloneRoutingTable()
	s.snapshot.Store(&snapshot)
	s.lastUpdateTime = time.Now().In(Location)
}

// register server
func (s *Server) registerServer() error {
	// register to http-gateway
	getResp, err := s.GetEtcdClient().Get(context.Background(), httpGatewayKey)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	if len(getResp.Kvs) < 1 {
		return util.ErrorfWithPos("HttpGateway addr not found in ETCD")
	}

	reqBody, err := json.Marshal(s.addr)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	req, err := http.NewRequest("POST", string(getResp.Kvs[0].Value), bytes.NewBuffer(reqBody))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return util.ErrorfWithPos("failed to send register request, status: %v, body: %v", resp.Status, resp.Body)
	}

	// register to etcd
	leaseResp, err := s.GetEtcdClient().Grant(context.Background(), 5)
	if err != nil {
		return util.ErrorWithPos(err)
	}

	_, err = s.GetEtcdClient().Put(context.Background(), "", s.addr.getAddrString(), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return util.ErrorWithPos(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.leaseCancelFunc = cancel
	aliveChan, err := s.GetEtcdClient().KeepAlive(ctx, leaseResp.ID)
	if err != nil {
		return util.ErrorWithPos(err)
	}

	go func() {
		for message := range aliveChan {
			if message == nil {
				mlog.Info("keepalive chan closed")
				return
			}
			mlog.Infof("received keepalive response: %v", message)
		}
	}()

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case providerRegisterPath:
		s.handleProviderRegister(w, r)
	case providerHeartbeatPath:
		s.handleProviderHeartbeat(w, r)
	case providerUnregisterPath:
		s.handleProviderUnregister(w, r)
	case consumerInitPath:
		s.handleConsumerInit(w, r)
	case consumerUpdatePath:
		s.handleConsumerUpdate(w, r)
	}
}

// provider register
func (s *Server) handleProviderRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	endpoint := &providerpb.RegisterRequest{}
	err := json.NewDecoder(r.Body).Decode(endpoint)
	if err != nil {
		mlog.Errorf("failed to decode request: %v", err)
		http.Error(w, wrapHTTPError(err), http.StatusBadRequest)
		return
	}
	idstr := r.Header.Get(eidHeader)
	eid, err := strconv.Atoi(idstr)
	if err != nil {
		mlog.Errorf("failed to convert str to int: %v", err)
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
		return
	}

	// 向etcd注册路由表
	err = s.registerProviderToEtcd(endpoint.GroupName, endpoint.HostName, &routingpb.Endpoint{
		Eid:    uint32(eid),
		Ip:     endpoint.Ip,
		Port:   endpoint.Port,
		Weight: uint32(endpoint.Weight),
	})
	if err != nil {
		mlog.Errorf("failed to register provider to etcd: %v", err)
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
		return
	}

	// 回复
	registerResp := &providerpb.RegisterReply{
		Eid: uint32(eid),
	}
	// 使用 json.NewEncoder 直接写入 http.ResponseWriter
	if err := json.NewEncoder(w).Encode(registerResp); err != nil {
		mlog.Errorf("failed to marshal and write response: %v", err)
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
	}
}

// provider heartbeat
func (s *Server) handleProviderHeartbeat(w http.ResponseWriter, r *http.Request) {
	endpoint := &providerpb.HeartbeatRequest{}
	err := json.NewDecoder(r.Body).Decode(endpoint)
	if err != nil {
		mlog.Errorf("failed to decode request: %v", err)
		http.Error(w, wrapHTTPError(err), http.StatusBadRequest)
		return
	}
	// keep alive
	err = s.keepAlive(endpoint.GroupName, endpoint.HostName, endpoint.Eid)
	if err != nil {
		mlog.Errorf("failed to keep alive endpoint: [%v/%v/%v]", endpoint.GroupName, endpoint.HostName, endpoint.Eid)
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
	}
}

// provider unregister
func (s *Server) handleProviderUnregister(w http.ResponseWriter, r *http.Request) {
	endpoint := &providerpb.UnregistRequest{}
	err := json.NewDecoder(r.Body).Decode(endpoint)
	if err != nil {
		mlog.Errorf("failed to decode request: %v", err)
		http.Error(w, wrapHTTPError(err), http.StatusBadRequest)
		return
	}
	// revoke lease
	err = s.revokeLease(endpoint.GroupName, endpoint.HostName, endpoint.Eid)
	if err != nil {
		mlog.Errorf("failed to unregister endpoint: [%v/%v/%v]", endpoint.GroupName, endpoint.HostName, endpoint.Eid)
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
	}
}

// consumer init
func (s *Server) handleConsumerInit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	routingTable := s.cloneRoutingTable()

	// 使用 json.NewEncoder 直接写入 http.ResponseWriter
	if err := json.NewEncoder(w).Encode(routingTable); err != nil {
		mlog.Errorf("failed to marshal and write response: %v", err)
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
	}
}

// consumer update
func (s *Server) handleConsumerUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

}
