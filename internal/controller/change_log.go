package controller

import (
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
)

type ChangeType int64

const (
	Add ChangeType = iota
	Update
	Delete
)

type ChangeRecord struct {
	ChangeType ChangeType
	GroupName  string
	HostName   string
	Endpoint   *routingpb.Endpoint
	Timestamp  int64
}

type ChangeRecords struct {
	Records []*ChangeRecord
	Version int64
}

func (l *ChangeRecords) insert(groupName string, hostName string, endpoint *routingpb.Endpoint) {
	now := time.Now().In(Location)
	l.Records = append(l.Records, &ChangeRecord{
		ChangeType: Add,
		GroupName:  groupName,
		HostName:   hostName,
		Endpoint:   endpoint,
		Timestamp:  now.Unix(),
	})
}

func (l *ChangeRecords) update(groupName string, hostName string, endpoint *routingpb.Endpoint) {
	now := time.Now().In(Location)
	l.Records = append(l.Records, &ChangeRecord{
		ChangeType: Update,
		GroupName:  groupName,
		HostName:   hostName,
		Endpoint:   endpoint,
		Timestamp:  now.Unix(),
	})
}

func (l *ChangeRecords) delete(groupName string, hostName string, endpoint *routingpb.Endpoint) {
	now := time.Now().In(Location)
	l.Records = append(l.Records, &ChangeRecord{
		ChangeType: Delete,
		GroupName:  groupName,
		HostName:   hostName,
		Endpoint:   endpoint,
		Timestamp:  now.Unix(),
	})
}
