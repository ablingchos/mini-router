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
)

type RoutingTable routingpb.RoutingTable

func (r *RoutingTable) insert(groupName string, hostName string, endpoint *routingpb.Endpoint) error {
	// check group
	if _, ok := r.Groups[groupName]; !ok {
		r.Groups[groupName] = &routingpb.Group{
			Name:  groupName,
			Hosts: make(map[string]*routingpb.Host),
		}
	}
	// check host
	group := r.Groups[groupName]
	if _, ok := group.Hosts[hostName]; !ok {
		group.Hosts[hostName] = &routingpb.Host{
			Name:      hostName,
			Endpoints: make(map[uint32]*routingpb.Endpoint),
		}
	}
	host := group.Hosts[hostName]
	// check eid
	eid := endpoint.Eid
	if _, ok := host.Endpoints[eid]; ok {
		return util.ErrorfWithPos("endpoint had registered, request failed: %v", endpoint)
	}

	host.Endpoints[eid] = endpoint
	mlog.Infof("add endpoint [%v/%v/%v]", groupName, hostName, eid)
	return nil
}

func (r *RoutingTable) update(groupName string, hostName string, endpoint *routingpb.Endpoint) error {
	// check group
	if _, ok := r.Groups[groupName]; !ok {
		return util.ErrorfWithPos("failed to update endpoint: group [%v] nost exist", groupName)
	}
	group := r.Groups[groupName]
	// check host
	if _, ok := group.Hosts[hostName]; !ok {
		return util.ErrorfWithPos("failed to update endpoint, host [%v] nost exist", hostName)
	}
	host := group.Hosts[hostName]
	// check eid
	eid := endpoint.Eid
	if _, ok := host.Endpoints[eid]; !ok {
		return util.ErrorfWithPos("failed to delete endpoint, endpoint [%v] nost exist", eid)
	}
	// check delete
	host.Endpoints[eid] = endpoint
	mlog.Infof("update endpoint [%v/%v/%v]", groupName, hostName, eid)
	return nil
}

func (r *RoutingTable) delete(groupName string, hostName string, eidStr string) error {
	// check group
	if _, ok := r.Groups[groupName]; !ok {
		return util.ErrorfWithPos("failed to delete endpoint: group [%v] nost exist", groupName)
	}
	group := r.Groups[groupName]
	// check host
	if _, ok := group.Hosts[hostName]; !ok {
		return util.ErrorfWithPos("failed to delete endpoint, host [%v] nost exist", hostName)
	}
	host := group.Hosts[hostName]
	// check eid
	eid, err := strconv.Atoi(eidStr)
	if err != nil {
		return util.ErrorWithPos(err)
	}
	if _, ok := host.Endpoints[uint32(eid)]; !ok {
		return util.ErrorfWithPos("failed to delete endpoint, endpoint [%v] nost exist", eid)
	}
	// check delete
	delete(host.Endpoints, uint32(eid))
	mlog.Infof("delete endpoint [%v/%v/%v]", groupName, hostName, eid)

	if len(host.Endpoints) == 0 {
		delete(group.Hosts, hostName)
		mlog.Infof("host is empty, delete host [%v/%v]", groupName, hostName)
		if len(group.Hosts) == 0 {
			delete(r.Groups, groupName)
			mlog.Infof("group is empty, delete group [%v]", groupName)
		}
	}
	return nil
}

func (r *RoutingTable) getLeaseID(groupName string, hostName string, eid uint32) (int64, error) {
	// check group
	if _, ok := r.Groups[groupName]; !ok {
		return 0, util.ErrorfWithPos("failed to delete endpoint: group [%v] nost exist", groupName)
	}
	group := r.Groups[groupName]
	// check host
	if _, ok := group.Hosts[hostName]; !ok {
		return 0, util.ErrorfWithPos("failed to delete endpoint, host [%v] nost exist", hostName)
	}
	host := group.Hosts[hostName]
	// check eid
	if _, ok := host.Endpoints[eid]; !ok {
		return 0, util.ErrorfWithPos("failed to delete endpoint, endpoint [%v] nost exist", eid)
	}

	leaseID := host.Endpoints[eid].LeaseId
	return leaseID, nil
}

func (s *Server) addEndpoint(key string, value string) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != routingTablePrefix {
		mlog.Warn("wrong format to add an endpoint", zap.Any("key", key), zap.Any("value", value))
		return
	}

	endpointInfo := &routingpb.Endpoint{}
	err := json.Unmarshal([]byte(value), endpointInfo)
	if err != nil {
		mlog.Warn("failed to unmarshal", zap.Any("key", key), zap.Any("value", value))
		return
	}
	// add endpoint
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.routingTable.insert(words[1], words[2], endpointInfo)
	if err != nil {
		mlog.Errorf("failed to add endpoint, err: %v", err)
	}
}

func (s *Server) updateEndpoint(key string, value string) {
	// split key
	words := strings.Split(key, "/")
	if len(words) != 4 || words[0] != routingTablePrefix {
		mlog.Warn("wrong format to add an endpoint", zap.Any("key", key), zap.Any("value", value))
		return
	}

	endpointInfo := &routingpb.Endpoint{}
	err := json.Unmarshal([]byte(value), endpointInfo)
	if err != nil {
		mlog.Warn("failed to unmarshal", zap.Any("key", key), zap.Any("value", value))
		return
	}
	// // update endpoint
	// s.mu.Lock()
	// defer s.mu.Unlock()
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
	_, err = s.etcdClient.KeepAliveOnce(context.Background(), clientv3.LeaseID(leaseID))
	if err != nil {
		return util.ErrorWithPos(err)
	}
	return nil
}
