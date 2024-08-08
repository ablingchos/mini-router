package controller

import (
	"context"

	"git.woa.com/kefuai/mini-router/pkg/proto/providerpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type HealthChecker struct {
	etcdClient clientv3.Client
	providerpb.UnimplementedProviderServiceServer
}

func (hc *HealthChecker) Heartbeat(ctx context.Context, req *providerpb.HeartbeatRequest) (*providerpb.HeartbeatReply, error) {
	// keep alive
	err := hc.keepAlive()
	if err != nil {
		mlog.Errorf("failed to keep alive endpoint: [%v/%v/%v]", req.GetGroupName(), req.GetHostName(), req.GetEid())
		return nil, err
	}
	return nil, nil
}

func (hc *HealthChecker) keepAlive(leaseID int64) error {
	_, err := hc.etcdClient.KeepAliveOnce(context.Background(), clientv3.LeaseID(leaseID))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}
