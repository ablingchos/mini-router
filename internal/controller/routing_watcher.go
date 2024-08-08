package controller

import clientv3 "go.etcd.io/etcd/client/v3"

type RoutingWatcher struct {
	etcdClient clientv3.Client
}
