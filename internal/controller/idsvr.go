package controller

// import (
// 	"sync/atomic"

// 	"git.woa.com/kefuai/mini-router/pkg/proto/controllerpb"
// 	"golang.org/x/net/context"
// )

// type IDGenerator struct {
// 	generator atomic.Int64

// 	controllerpb.UnimplementedIdServiceServer
// }

// func (i *IDGenerator) GenerateId(context.Context, *controllerpb.GenerateIdRequest) (*controllerpb.GenerateIdReply, error) {
// 	return &controllerpb.GenerateIdReply{
// 		Eid: i.generator.Add(1),
// 	}, nil
// }
