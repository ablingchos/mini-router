package controller

import (
	"sync"
	"time"

	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"google.golang.org/protobuf/proto"
)

type LogWriter struct {
	records *routingpb.ChangeRecords
	mu      sync.Mutex
}

func NewLogWriter() *LogWriter {
	return &LogWriter{}
}

func (l *LogWriter) newRecords(version int64) {
	l.records = &routingpb.ChangeRecords{
		Records: make([]*routingpb.ChangeRecord, 0),
		Version: version,
	}
}

func (l *LogWriter) flushRecords(version int64) *routingpb.ChangeRecords {
	l.mu.Lock()
	defer l.mu.Unlock()
	cloned, _ := proto.Clone(l.records).(*routingpb.ChangeRecords)
	l.newRecords(version)
	return cloned
}

func (l *LogWriter) insert(groupName string, hostName string, endpoint *routingpb.Endpoint) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().In(Location)
	l.records.Records = append(l.records.Records, &routingpb.ChangeRecord{
		Type:      routingpb.ChangeType_add,
		GroupName: groupName,
		HostName:  hostName,
		Endpoint:  endpoint,
		TimeStamp: now.Unix(),
	})
}

func (l *LogWriter) update(groupName string, hostName string, endpoint *routingpb.Endpoint) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().In(Location)
	l.records.Records = append(l.records.Records, &routingpb.ChangeRecord{
		Type:      routingpb.ChangeType_update,
		GroupName: groupName,
		HostName:  hostName,
		Endpoint:  endpoint,
		TimeStamp: now.Unix(),
	})
}

func (l *LogWriter) delete(groupName string, hostName string, endpoint *routingpb.Endpoint) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().In(Location)
	l.records.Records = append(l.records.Records, &routingpb.ChangeRecord{
		Type:      routingpb.ChangeType_delete,
		GroupName: groupName,
		HostName:  hostName,
		Endpoint:  endpoint,
		TimeStamp: now.Unix(),
	})
}
