package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/providerpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	routingTablePrefix = "routing"
)

type Server struct {
	etcdClient      *clientv3.Client
	leaseCancelFunc context.CancelFunc
	watchCancelFunc context.CancelFunc
	stopCh          chan struct{}
	addr            *IpPort
	routingTable    *RoutingTable
	mu              sync.RWMutex
}

type EndpointInfo struct {
	GroupName string `json:"group_name"`
	HostName  string `json:"host_name"`
	Eid       uint32 `json:"eid"`
	LeaseID   int64  `json:"lease_id"`
	Healthy   bool   `json:"healthy"`
}

func NewServer(port string) (*Server, error) {
	server := &Server{
		addr:         &IpPort{},
		stopCh:       make(chan struct{}),
		routingTable: &RoutingTable{},
	}
	var err error
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
	err := s.register()
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

func (s *Server) watchLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	s.watchCancelFunc = cancel
	watchChan := s.GetEtcdClient().Watch(ctx, "/"+routingTablePrefix, clientv3.WithPrefix())
	for watchResp := range watchChan {
		for _, ev := range watchResp.Events {
			if ev.IsCreate() {
				s.Insert(string(ev.Kv.Key), string(ev.Kv.Value))
			} else if ev.Type == mvccpb.DELETE {

			}
		}
	}
}

// register server
func (s *Server) register() error {
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
	err := json.NewDecoder(r.Body).Decode(&endpoint)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	idstr := r.Header.Get(eidHeader)
	eid, err := strconv.Atoi(idstr)
	if err != nil {
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
		return
	}

	// 向etcd注册路由表
	s.registerProviderToEtcd("/"+routingTablePrefix+"/"+strconv.Itoa(eid), &EndpointInfo{
		GroupName: endpoint.GroupName,
		HostName:  endpoint.HostName,
		Eid:       uint32(eid),
	})

	// 回复
	ret := &providerpb.RegisterReply{
		Eid: uint32(eid),
	}
	respBytes, err := json.Marshal(ret)
	if err != nil {
		http.Error(w, wrapHTTPError(err), http.StatusInternalServerError)
		return
	}
	w.Write(respBytes)
}

// provider heartbeat
func (s *Server) handleProviderHeartbeat(w http.ResponseWriter, r *http.Request) {
}

// provider unregister
func (s *Server) handleProviderUnregister(w http.ResponseWriter, r *http.Request) {
}

// consumer init
func (s *Server) handleConsumerInit(w http.ResponseWriter, r *http.Request) {
}

// consumer update
func (s *Server) handleConsumerUpdate(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) registerProviderToEtcd(heartbeatKey string, endpoint *providerpb.RegisterRequest) error {
	keys := []string{endpoint.GroupName, endpoint.HostName}
	endpointKey := "/" + strings.Join(keys, "/")

	leaseResp, err := s.GetEtcdClient().Grant(context.Background(), 5)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	endpoint.LeaseID = int64(leaseResp.ID)

	bytes, err := json.Marshal(endpoint)
	if err != nil {
		return util.ErrorWithPos(err)
	}

	_, err = s.GetEtcdClient().Put(context.Background(), endpointKey, string(bytes))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	_, err = s.GetEtcdClient().Put(context.Background(), heartbeatKey, time.Now().Truncate(time.Second).String(), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}
