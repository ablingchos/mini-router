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
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	idMutexPrefix = "/mutex"
	idPrefix      = "/id"
)

type RegisterServer struct {
	etcdClient *clientv3.Client
	eidNumber  atomic.Int64

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
	eid, err := r.generateEid(ctx)
	if err != nil {
		mlog.Errorf("failed to generate new gid: %v", err)
		return nil, util.ErrorWithPos(err)
	}
	// 在etcd中注册的key，四段式: "/routing/group1/host1/eid"
	keys := []string{routingTablePrefix, req.GroupName, req.HostName, strconv.Itoa(int(eid))}
	endpointKey := "/" + strings.Join(keys, "/")
	// 向etcd创建lease
	leaseResp, err := r.etcdClient.Grant(ctx, int64(req.GetTimeout()))
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
	_, err = r.etcdClient.Put(ctx, endpointKey, string(bytes), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}

	mlog.Infof("[%v/%v/%v] register finished", req.GetGroupName(), req.GetHostName(), eid)
	return &providerpb.RegisterReply{
		Eid:     eid,
		LeaseId: int64(leaseResp.ID),
	}, nil
}

func (r *RegisterServer) Heartbeat(ctx context.Context, req *providerpb.HeartbeatRequest) (*providerpb.HeartbeatReply, error) {
	// keep alive
	_, err := r.etcdClient.KeepAliveOnce(ctx, clientv3.LeaseID(req.GetLeaseId()))
	if err != nil {
		mlog.Errorf("failed to keep alive endpoint: [%v/%v/%v]", req.GetGroupName(), req.GetHostName(), req.GetEid())
		return nil, util.ErrorWithPos(err)
	}
	mlog.Infof("[%v/%v/%v] heartbeat finished", req.GetGroupName(), req.GetHostName(), req.GetEid())
	return &providerpb.HeartbeatReply{}, nil
}

func (r *RegisterServer) Unregister(ctx context.Context, req *providerpb.UnregisterRequest) (*providerpb.UnregisterReply, error) {
	_, err := r.etcdClient.Revoke(ctx, clientv3.LeaseID(req.GetLeaseId()))
	if err != nil {
		return nil, util.ErrorWithPos(err)
	}
	mlog.Infof("[%v/%v/%v] unregister finished", req.GetGroupName(), req.GetHostName(), req.GetEid())
	return &providerpb.UnregisterReply{}, nil
}

func (r *RegisterServer) generateEid(ctx context.Context) (int64, error) {
	// 创建一个分布式锁会话
	session, err := concurrency.NewSession(r.etcdClient)
	if err != nil {
		return 0, util.ErrorWithPos(err)
	}
	defer session.Close()
	// 创建一个分布式锁
	mutex := concurrency.NewMutex(session, idMutexPrefix)
	// 获取锁
	if err := mutex.Lock(ctx); err != nil {
		return 0, util.ErrorWithPos(err)
	}
	defer func() {
		if err := mutex.Unlock(ctx); err != nil {
			mlog.Errorf("Failed to release lock: %v", err)
		}
	}()
	// 使用事务初始化值
	txn := r.etcdClient.Txn(ctx)
	// 事务操作：如果键不存在，则初始化它
	txnResp, err := txn.If(
		clientv3.Compare(clientv3.Version(idPrefix), "=", 0),
	).Then(
		clientv3.OpPut(idPrefix, strconv.Itoa(1)),
	).Else(
		clientv3.OpGet(idPrefix),
	).Commit()

	if err != nil {
		return 0, util.ErrorWithPos(err)
	}
	// 获取并更新eid
	eid := int64(1)
	if txnResp.Succeeded {
		mlog.Infof("created eid key for the first time")
	} else {
		value, err := strconv.Atoi(string(txnResp.Responses[0].GetResponseRange().Kvs[0].Value))
		if err != nil {
			return 0, util.ErrorWithPos(err)
		}
		eid = int64(value) + 1

		// 更新值
		_, err = r.etcdClient.Put(ctx, idPrefix, strconv.Itoa(int(eid)))
		if err != nil {
			return 0, util.ErrorWithPos(err)
		}
	}

	return eid, nil
}
