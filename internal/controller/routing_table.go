package controller

import (
	"encoding/json"

	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
)

type RoutingTable routingpb.RoutingTable

func (r *RoutingTable) Insert(endpoint *EndpointInfo) {
	groupName := endpoint.GroupName
	if _, ok := r.Groups[groupName]; !ok {
		r.Groups[groupName] = &routingpb.Group{
			Name:  groupName,
			Hosts: make(map[string]*routingpb.Host),
		}
	}

	group := r.Groups[groupName]
	hostName := endpoint.HostName
	if _, ok := group.Hosts[hostName]; !ok {
		group.Hosts[hostName] = &routingpb.Host{
			Name:      hostName,
			Endpoints: make(map[uint32]*routingpb.Endpoint),
		}
	}

	host := group.Hosts[hostName]
	eid := endpoint.Eid
	if _, ok := host.Endpoints[eid]; ok {
		mlog.Warn("endpoint had registered, request faile", zap.Any("endpoint info", endpoint))
		return
	}

	host.Endpoints[eid] = &routingpb.Endpoint{
		Eid: eid,
		Ip:  endpoint.Eid,
	}
}

func (s *Server) insert(key string, value string) {
	endpointInfo := &EndpointInfo{}
	err := json.Unmarshal([]byte(value), endpointInfo)
	if err != nil {
		mlog.Warn("failed to unmarshal", zap.Any("key", key), zap.Any("value", value))
		return
	}

	s.mu.Lock()
	s.routingTable.Insert(endpointInfo)
}
