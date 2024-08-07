package controller

import (
	"encoding/json"

	"git.woa.com/kefuai/mini-router/pkg/proto/routingpb"
	"git.woa.com/mfcn/ms-go/pkg/mlog"
	"go.uber.org/zap"
)

type RoutingTable routingpb.RoutingTable

func (r *RoutingTable) Insert() {

}

func (s *Server) Insert(key string, value string) {
	endpointInfo := &EndpointInfo{}
	err := json.Unmarshal([]byte(value), endpointInfo)
	if err != nil {
		mlog.Warn("failed to unmarshal", zap.Any("key", key), zap.Any("value", value))
		return
	}

	s.mu.Lock()
	s.routingTable.Insert()
}
