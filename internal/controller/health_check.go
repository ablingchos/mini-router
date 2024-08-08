package controller

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/providerpb"
	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type RegisterServer struct {
	etcdClient *clientv3.Client
	eidNumber  atomic.Uint32

	providerpb.UnimplementedProviderServiceServer
}

func NewRegisterServer() (*RegisterServer, error) {
	registerServer := &RegisterServer{}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{etcdUri},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	registerServer.etcdClient = client
	return registerServer, nil
}

func (r *RegisterServer) Register(ctx context.Context, req *providerpb.RegisterRequest) (*providerpb.RegisterReply, error) {
	eid := r.eidNumber.Add(1)
	// 在etcd中注册的key，四段式: "/routing/group1/host1/eid"
	keys := []string{routingTablePrefix, req.GroupName, req.HostName, strconv.Itoa(int(eid))}
	endpointKey := "/" + strings.Join(keys, "/")
	// 向etcd创建lease
	leaseResp, err := r.etcdClient.Grant(context.Background(), int64(req.GetTimeout()))
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	endpoint := &routingpb.Endpoint{
		Eid:     eid,
		Ip:      req.GetIp(),
		Port:    req.GetPort(),
		Weight:  req.GetWeight(),
		LeaseId: int64(leaseResp.ID),
	}

	bytes, err := json.Marshal(endpoint)
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	_, err = r.etcdClient.Put(context.Background(), endpointKey, string(bytes), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}

	return &providerpb.RegisterReply{
		Eid:     eid,
		LeaseId: int64(leaseResp.ID),
	}, nil
}

func (r *RegisterServer) Heartbeat(ctx context.Context, req *providerpb.HeartbeatRequest) (*providerpb.HeartbeatReply, error) {
	// keep alive
	_, err := r.etcdClient.KeepAliveOnce(context.Background(), clientv3.LeaseID(req.GetLeaseId()))
	if err != nil {
		mlog.Errorf("failed to keep alive endpoint: [%v/%v/%v]", req.GetGroupName(), req.GetHostName(), req.GetEid())
		return nil, util.ErrorWithPos(err)
	}
	return &providerpb.HeartbeatReply{}, nil
}
