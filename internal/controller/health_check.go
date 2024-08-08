package controller

import (
	"git.woa.com/kefuai/mini-router/pkg/proto/providerpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type HealthChecker struct {
	etcdClient clientv3.Client
}

func (hc *HealthChecker) Heartbeat(req *providerpb.HeartbeatRequest) {

}
