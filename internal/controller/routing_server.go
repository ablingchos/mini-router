package controller

import (
	"context"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/util"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type RoutingServer struct {
	etcdClient   *clientv3.Client
	routingTable *routingpb.RoutingTable

	watchCancelFunc context.CancelFunc
}

func NewRoutingServer() (*RoutingServer, error) {
	server := &RoutingServer{
		routingTable: &routingpb.RoutingTable{
			Groups: make(map[string]*routingpb.Group),
		},
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

func (s *RoutingServer) Run() {

}
