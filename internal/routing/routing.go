package routing

import (
	"strconv"

	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"git.woa.com/mfcn/ms-go/pkg/util"
)

type RoutingTable routingpb.RoutingTable

func (r *RoutingTable) Insert(groupName string, hostName string, endpoint *routingpb.Endpoint) error {
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

func (r *RoutingTable) Update(groupName string, hostName string, endpoint *routingpb.Endpoint) error {
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

func (r *RoutingTable) Delete(groupName string, hostName string, eidStr string) error {
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

func (r *RoutingTable) GetLeaseID(groupName string, hostName string, eid uint32) (int64, error) {
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
