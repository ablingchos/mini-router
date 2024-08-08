package controller

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func (s *Server) addEndpoint(key string, endpointInfo *routingpb.Endpoint) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != routingTablePrefix {
		mlog.Warn("wrong format to add an endpoint", zap.Any("key", key), zap.Any("value", endpointInfo))
		return
	}

	// endpointInfo := &routingpb.Endpoint{}
	// err := json.Unmarshal([]byte(value), endpointInfo)
	// if err != nil {
	// 	mlog.Warn("failed to unmarshal", zap.Any("key", key), zap.Any("value", value))
	// 	return
	// }
	// add endpoint
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.routingTable.insert(words[1], words[2], endpointInfo)
	if err != nil {
		mlog.Errorf("failed to add endpoint, err: %v", err)
	}
}

func (s *Server) updateEndpoint(key string, endpointInfo *routingpb.Endpoint) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != routingTablePrefix {
		mlog.Warn("wrong format to add an endpoint", zap.Any("key", key), zap.Any("value", endpointInfo))
		return
	}

	// endpointInfo := &routingpb.Endpoint{}
	// err := json.Unmarshal([]byte(value), endpointInfo)
	// if err != nil {
	// 	mlog.Warn("failed to unmarshal", zap.Any("key", key), zap.Any("value", value))
	// 	return
	// }
	// // update endpoint
	s.mu.Lock()
	defer s.mu.Unlock()
	// err = s.routingTable.update(words[1], words[2], endpointInfo)
	// if err != nil {
	// 	mlog.Errorf("failed to update endpoint, err: %v", err)
	// }
	mlog.Debugf("update endpoint [%v/%v/%v]", words[1], words[2], words[3])
}

func (s *Server) deleteEndpoint(key string) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != routingTablePrefix {
		mlog.Warn("wrong format to delete an endpoint", zap.Any("key", key))
		return
	}
	// delete key
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.routingTable.delete(words[1], words[2], words[3])
	if err != nil {
		mlog.Errorf("failed to delete endpoint, err: %v", err)
	}
}

func (s *Server) keepAlive(groupName string, hostName string, eid uint32) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	leaseID, err := s.routingTable.getLeaseID(groupName, hostName, eid)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	_, err = s.GetEtcdClient().KeepAliveOnce(context.Background(), clientv3.LeaseID(leaseID))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

func (s *Server) registerProviderToEtcd(groupName string, hostName string, endpoint *routingpb.Endpoint) error {
	// 在etcd中注册的key，四段式: "/routing/group1/host1/eid"
	keys := []string{routingTablePrefix, groupName, hostName, strconv.Itoa(int(endpoint.Eid))}
	endpointKey := "/" + strings.Join(keys, "/")

	leaseResp, err := s.GetEtcdClient().Grant(context.Background(), 5)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	endpoint.LeaseId = int64(leaseResp.ID)

	bytes, err := json.Marshal(endpoint)
	if err != nil {
		return util.ErrorWithPos(err)
	}

	_, err = s.GetEtcdClient().Put(context.Background(), endpointKey, string(bytes), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

func (s *Server) revokeLease(groupName string, hostName string, eid uint32) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	leaseID, err := s.routingTable.getLeaseID(groupName, hostName, eid)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	_, err = s.GetEtcdClient().Revoke(context.Background(), clientv3.LeaseID(leaseID))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}

func (s *Server) cloneRoutingTable() *routingpb.RoutingTable {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cloned, _ := proto.Clone((*routingpb.RoutingTable)(s.routingTable)).(*routingpb.RoutingTable)
	return cloned
}
