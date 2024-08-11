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
	return &LogWriter{
		records: &routingpb.ChangeRecords{
			Records: make([]*routingpb.ChangeRecord, 0),
		},
	}
}

func (l *LogWriter) flush(version int64) *routingpb.ChangeRecords {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records.Version = version
	cloned, _ := proto.Clone(l.records).(*routingpb.ChangeRecords)
	l.records = &routingpb.ChangeRecords{
		Records: make([]*routingpb.ChangeRecord, 0),
	}
	return cloned
}

func (l *LogWriter) write(groupName string, hostName string, endpoint *routingpb.Endpoint, method routingpb.ChangeType, loc *time.Location) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().In(loc)
	l.records.Records = append(l.records.Records, &routingpb.ChangeRecord{
		Type:      method,
		GroupName: groupName,
		HostName:  hostName,
		Endpoint:  endpoint,
		TimeStamp: now.Unix(),
	})
}
