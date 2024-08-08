// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0(customized by red@2023-12-12)
// - protoc             (unknown)
// source: pkg/proto/controllerpb/controller.proto

package controllerpb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// IdServiceClient is the client API for IdService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type IdServiceClient interface {
	GenerateId(ctx context.Context, in *GenerateIdRequest, opts ...grpc.CallOption) (*GenerateIdReply, error)
}

type idServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewIdServiceClient(cc grpc.ClientConnInterface) IdServiceClient {
	return &idServiceClient{cc}
}

func (c *idServiceClient) GenerateId(ctx context.Context, in *GenerateIdRequest, opts ...grpc.CallOption) (*GenerateIdReply, error) {
	out := new(GenerateIdReply)
	err := c.cc.Invoke(ctx, "/controllerpb.IdService/GenerateId", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// IdServiceServer is the server API for IdService service.
// All implementations must embed UnimplementedIdServiceServer
// for forward compatibility
type IdServiceServer interface {
	GenerateId(context.Context, *GenerateIdRequest) (*GenerateIdReply, error)
	mustEmbedUnimplementedIdServiceServer()
}

// UnimplementedIdServiceServer must be embedded to have forward compatible implementations.
type UnimplementedIdServiceServer struct {
}

func (UnimplementedIdServiceServer) GenerateId(context.Context, *GenerateIdRequest) (*GenerateIdReply, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GenerateId not implemented")
}
func (UnimplementedIdServiceServer) mustEmbedUnimplementedIdServiceServer() {}

// UnsafeIdServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to IdServiceServer will
// result in compilation errors.
type UnsafeIdServiceServer interface {
	mustEmbedUnimplementedIdServiceServer()
}

func RegisterIdServiceServer(s grpc.ServiceRegistrar, srv IdServiceServer) {
	s.RegisterService(&IdService_ServiceDesc, srv)
}

func _IdService_GenerateId_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GenerateIdRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IdServiceServer).GenerateId(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/controllerpb.IdService/GenerateId",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IdServiceServer).GenerateId(ctx, req.(*GenerateIdRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// IdService_ServiceDesc is the grpc.ServiceDesc for IdService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var IdService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "controllerpb.IdService",
	HandlerType: (*IdServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GenerateId",
			Handler:    _IdService_GenerateId_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pkg/proto/controllerpb/controller.proto",
}
