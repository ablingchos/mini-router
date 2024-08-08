// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: pkg/proto/routingpb/routing.proto

package routingpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type LoadBalancer int32

const (
	LoadBalancer_random          LoadBalancer = 0 // 随机
	LoadBalancer_consistent_hash LoadBalancer = 1 // 一致性哈希
	LoadBalancer_weight          LoadBalancer = 2 // 权重
	LoadBalancer_destination     LoadBalancer = 3 // 指定目标
)

// Enum value maps for LoadBalancer.
var (
	LoadBalancer_name = map[int32]string{
		0: "random",
		1: "consistent_hash",
		2: "weight",
		3: "destination",
	}
	LoadBalancer_value = map[string]int32{
		"random":          0,
		"consistent_hash": 1,
		"weight":          2,
		"destination":     3,
	}
)

func (x LoadBalancer) Enum() *LoadBalancer {
	p := new(LoadBalancer)
	*p = x
	return p
}

func (x LoadBalancer) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LoadBalancer) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_proto_routingpb_routing_proto_enumTypes[0].Descriptor()
}

func (LoadBalancer) Type() protoreflect.EnumType {
	return &file_pkg_proto_routingpb_routing_proto_enumTypes[0]
}

func (x LoadBalancer) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LoadBalancer.Descriptor instead.
func (LoadBalancer) EnumDescriptor() ([]byte, []int) {
	return file_pkg_proto_routingpb_routing_proto_rawDescGZIP(), []int{0}
}

type Match int32

const (
	Match_prefix Match = 0 // 前缀匹配
	Match_exact  Match = 1 // 完全匹配
)

// Enum value maps for Match.
var (
	Match_name = map[int32]string{
		0: "prefix",
		1: "exact",
	}
	Match_value = map[string]int32{
		"prefix": 0,
		"exact":  1,
	}
)

func (x Match) Enum() *Match {
	p := new(Match)
	*p = x
	return p
}

func (x Match) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Match) Descriptor() protoreflect.EnumDescriptor {
	return file_pkg_proto_routingpb_routing_proto_enumTypes[1].Descriptor()
}

func (Match) Type() protoreflect.EnumType {
	return &file_pkg_proto_routingpb_routing_proto_enumTypes[1]
}

func (x Match) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Match.Descriptor instead.
func (Match) EnumDescriptor() ([]byte, []int) {
	return file_pkg_proto_routingpb_routing_proto_rawDescGZIP(), []int{1}
}

// 三级结构：group（组）->host（服务）->endpoint（节点）
type RoutingTable struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Groups map[string]*Group `protobuf:"bytes,1,rep,name=groups,proto3" json:"groups,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *RoutingTable) Reset() {
	*x = RoutingTable{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RoutingTable) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RoutingTable) ProtoMessage() {}

func (x *RoutingTable) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RoutingTable.ProtoReflect.Descriptor instead.
func (*RoutingTable) Descriptor() ([]byte, []int) {
	return file_pkg_proto_routingpb_routing_proto_rawDescGZIP(), []int{0}
}

func (x *RoutingTable) GetGroups() map[string]*Group {
	if x != nil {
		return x.Groups
	}
	return nil
}

type Group struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name  string           `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`                                                                                           // 路由组名称（类似namespace的概念）
	Hosts map[string]*Host `protobuf:"bytes,2,rep,name=hosts,proto3" json:"hosts,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"` // host表
}

func (x *Group) Reset() {
	*x = Group{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Group) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Group) ProtoMessage() {}

func (x *Group) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Group.ProtoReflect.Descriptor instead.
func (*Group) Descriptor() ([]byte, []int) {
	return file_pkg_proto_routingpb_routing_proto_rawDescGZIP(), []int{1}
}

func (x *Group) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Group) GetHosts() map[string]*Host {
	if x != nil {
		return x.Hosts
	}
	return nil
}

type Host struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name        string              `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`                                                                                                    // host名
	MatchRule   *MatchRule          `protobuf:"bytes,2,opt,name=match_rule,json=matchRule,proto3" json:"match_rule,omitempty"`                                                                         // 键值匹配规则
	RoutingRule LoadBalancer        `protobuf:"varint,3,opt,name=routing_rule,json=routingRule,proto3,enum=routingpb.LoadBalancer" json:"routing_rule,omitempty"`                                      // 标准路由规则
	Endpoints   map[int64]*Endpoint `protobuf:"bytes,4,rep,name=endpoints,proto3" json:"endpoints,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"` // 节点表
}

func (x *Host) Reset() {
	*x = Host{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Host) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Host) ProtoMessage() {}

func (x *Host) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Host.ProtoReflect.Descriptor instead.
func (*Host) Descriptor() ([]byte, []int) {
	return file_pkg_proto_routingpb_routing_proto_rawDescGZIP(), []int{2}
}

func (x *Host) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Host) GetMatchRule() *MatchRule {
	if x != nil {
		return x.MatchRule
	}
	return nil
}

func (x *Host) GetRoutingRule() LoadBalancer {
	if x != nil {
		return x.RoutingRule
	}
	return LoadBalancer_random
}

func (x *Host) GetEndpoints() map[int64]*Endpoint {
	if x != nil {
		return x.Endpoints
	}
	return nil
}

type Endpoint struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Eid     int64  `protobuf:"varint,1,opt,name=eid,proto3" json:"eid,omitempty"`                        // 节点编号
	Ip      string `protobuf:"bytes,2,opt,name=ip,proto3" json:"ip,omitempty"`                           // ip
	Port    string `protobuf:"bytes,3,opt,name=port,proto3" json:"port,omitempty"`                       // port
	Weight  int64  `protobuf:"varint,4,opt,name=weight,proto3" json:"weight,omitempty"`                  // 权重
	Timeout int64  `protobuf:"varint,5,opt,name=timeout,proto3" json:"timeout,omitempty"`                // 心跳超时时间
	LeaseId int64  `protobuf:"varint,6,opt,name=lease_id,json=leaseId,proto3" json:"lease_id,omitempty"` // 租约id
}

func (x *Endpoint) Reset() {
	*x = Endpoint{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Endpoint) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Endpoint) ProtoMessage() {}

func (x *Endpoint) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Endpoint.ProtoReflect.Descriptor instead.
func (*Endpoint) Descriptor() ([]byte, []int) {
	return file_pkg_proto_routingpb_routing_proto_rawDescGZIP(), []int{3}
}

func (x *Endpoint) GetEid() int64 {
	if x != nil {
		return x.Eid
	}
	return 0
}

func (x *Endpoint) GetIp() string {
	if x != nil {
		return x.Ip
	}
	return ""
}

func (x *Endpoint) GetPort() string {
	if x != nil {
		return x.Port
	}
	return ""
}

func (x *Endpoint) GetWeight() int64 {
	if x != nil {
		return x.Weight
	}
	return 0
}

func (x *Endpoint) GetTimeout() int64 {
	if x != nil {
		return x.Timeout
	}
	return 0
}

func (x *Endpoint) GetLeaseId() int64 {
	if x != nil {
		return x.LeaseId
	}
	return 0
}

type MatchRule struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Match   Match  `protobuf:"varint,1,opt,name=match,proto3,enum=routingpb.Match" json:"match,omitempty"` // 匹配方式
	Content string `protobuf:"bytes,2,opt,name=content,proto3" json:"content,omitempty"`                   // 匹配内容
}

func (x *MatchRule) Reset() {
	*x = MatchRule{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MatchRule) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MatchRule) ProtoMessage() {}

func (x *MatchRule) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_proto_routingpb_routing_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MatchRule.ProtoReflect.Descriptor instead.
func (*MatchRule) Descriptor() ([]byte, []int) {
	return file_pkg_proto_routingpb_routing_proto_rawDescGZIP(), []int{4}
}

func (x *MatchRule) GetMatch() Match {
	if x != nil {
		return x.Match
	}
	return Match_prefix
}

func (x *MatchRule) GetContent() string {
	if x != nil {
		return x.Content
	}
	return ""
}

var File_pkg_proto_routingpb_routing_proto protoreflect.FileDescriptor

var file_pkg_proto_routingpb_routing_proto_rawDesc = []byte{
	0x0a, 0x21, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x72, 0x6f, 0x75, 0x74,
	0x69, 0x6e, 0x67, 0x70, 0x62, 0x2f, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x09, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x22, 0x98,
	0x01, 0x0a, 0x0c, 0x52, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x54, 0x61, 0x62, 0x6c, 0x65, 0x12,
	0x3b, 0x0a, 0x06, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x23, 0x2e, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x2e, 0x52, 0x6f, 0x75, 0x74,
	0x69, 0x6e, 0x67, 0x54, 0x61, 0x62, 0x6c, 0x65, 0x2e, 0x47, 0x72, 0x6f, 0x75, 0x70, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x06, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x73, 0x1a, 0x4b, 0x0a, 0x0b,
	0x47, 0x72, 0x6f, 0x75, 0x70, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x26, 0x0a,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10, 0x2e, 0x72,
	0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x2e, 0x47, 0x72, 0x6f, 0x75, 0x70, 0x52, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x99, 0x01, 0x0a, 0x05, 0x47, 0x72,
	0x6f, 0x75, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x31, 0x0a, 0x05, 0x68, 0x6f, 0x73, 0x74, 0x73,
	0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67,
	0x70, 0x62, 0x2e, 0x47, 0x72, 0x6f, 0x75, 0x70, 0x2e, 0x48, 0x6f, 0x73, 0x74, 0x73, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x52, 0x05, 0x68, 0x6f, 0x73, 0x74, 0x73, 0x1a, 0x49, 0x0a, 0x0a, 0x48, 0x6f,
	0x73, 0x74, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x25, 0x0a, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x72, 0x6f, 0x75, 0x74,
	0x69, 0x6e, 0x67, 0x70, 0x62, 0x2e, 0x48, 0x6f, 0x73, 0x74, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x9c, 0x02, 0x0a, 0x04, 0x48, 0x6f, 0x73, 0x74, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x33, 0x0a, 0x0a, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x5f, 0x72, 0x75, 0x6c, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67,
	0x70, 0x62, 0x2e, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x52, 0x75, 0x6c, 0x65, 0x52, 0x09, 0x6d, 0x61,
	0x74, 0x63, 0x68, 0x52, 0x75, 0x6c, 0x65, 0x12, 0x3a, 0x0a, 0x0c, 0x72, 0x6f, 0x75, 0x74, 0x69,
	0x6e, 0x67, 0x5f, 0x72, 0x75, 0x6c, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x17, 0x2e,
	0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x2e, 0x4c, 0x6f, 0x61, 0x64, 0x42, 0x61,
	0x6c, 0x61, 0x6e, 0x63, 0x65, 0x72, 0x52, 0x0b, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x52,
	0x75, 0x6c, 0x65, 0x12, 0x3c, 0x0a, 0x09, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x73,
	0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67,
	0x70, 0x62, 0x2e, 0x48, 0x6f, 0x73, 0x74, 0x2e, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74,
	0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x09, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74,
	0x73, 0x1a, 0x51, 0x0a, 0x0e, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x73, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x29, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x70, 0x62,
	0x2e, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x3a, 0x02, 0x38, 0x01, 0x22, 0x8d, 0x01, 0x0a, 0x08, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e,
	0x74, 0x12, 0x10, 0x0a, 0x03, 0x65, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x03,
	0x65, 0x69, 0x64, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x02, 0x69, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68,
	0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12,
	0x18, 0x0a, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x12, 0x19, 0x0a, 0x08, 0x6c, 0x65, 0x61,
	0x73, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x6c, 0x65, 0x61,
	0x73, 0x65, 0x49, 0x64, 0x22, 0x4d, 0x0a, 0x09, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x52, 0x75, 0x6c,
	0x65, 0x12, 0x26, 0x0a, 0x05, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x10, 0x2e, 0x72, 0x6f, 0x75, 0x74, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x2e, 0x4d, 0x61, 0x74,
	0x63, 0x68, 0x52, 0x05, 0x6d, 0x61, 0x74, 0x63, 0x68, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6e,
	0x74, 0x65, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74,
	0x65, 0x6e, 0x74, 0x2a, 0x4c, 0x0a, 0x0c, 0x4c, 0x6f, 0x61, 0x64, 0x42, 0x61, 0x6c, 0x61, 0x6e,
	0x63, 0x65, 0x72, 0x12, 0x0a, 0x0a, 0x06, 0x72, 0x61, 0x6e, 0x64, 0x6f, 0x6d, 0x10, 0x00, 0x12,
	0x13, 0x0a, 0x0f, 0x63, 0x6f, 0x6e, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x74, 0x5f, 0x68, 0x61,
	0x73, 0x68, 0x10, 0x01, 0x12, 0x0a, 0x0a, 0x06, 0x77, 0x65, 0x69, 0x67, 0x68, 0x74, 0x10, 0x02,
	0x12, 0x0f, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x74, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x10,
	0x03, 0x2a, 0x1e, 0x0a, 0x05, 0x4d, 0x61, 0x74, 0x63, 0x68, 0x12, 0x0a, 0x0a, 0x06, 0x70, 0x72,
	0x65, 0x66, 0x69, 0x78, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x65, 0x78, 0x61, 0x63, 0x74, 0x10,
	0x01, 0x42, 0x34, 0x5a, 0x32, 0x67, 0x69, 0x74, 0x2e, 0x77, 0x6f, 0x61, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x6b, 0x65, 0x66, 0x75, 0x61, 0x69, 0x2f, 0x6d, 0x69, 0x6e, 0x69, 0x2d, 0x72, 0x6f, 0x75,
	0x74, 0x65, 0x72, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x72, 0x6f,
	0x75, 0x74, 0x69, 0x6e, 0x67, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_proto_routingpb_routing_proto_rawDescOnce sync.Once
	file_pkg_proto_routingpb_routing_proto_rawDescData = file_pkg_proto_routingpb_routing_proto_rawDesc
)

func file_pkg_proto_routingpb_routing_proto_rawDescGZIP() []byte {
	file_pkg_proto_routingpb_routing_proto_rawDescOnce.Do(func() {
		file_pkg_proto_routingpb_routing_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_proto_routingpb_routing_proto_rawDescData)
	})
	return file_pkg_proto_routingpb_routing_proto_rawDescData
}

var file_pkg_proto_routingpb_routing_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_pkg_proto_routingpb_routing_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_pkg_proto_routingpb_routing_proto_goTypes = []interface{}{
	(LoadBalancer)(0),    // 0: routingpb.LoadBalancer
	(Match)(0),           // 1: routingpb.Match
	(*RoutingTable)(nil), // 2: routingpb.RoutingTable
	(*Group)(nil),        // 3: routingpb.Group
	(*Host)(nil),         // 4: routingpb.Host
	(*Endpoint)(nil),     // 5: routingpb.Endpoint
	(*MatchRule)(nil),    // 6: routingpb.MatchRule
	nil,                  // 7: routingpb.RoutingTable.GroupsEntry
	nil,                  // 8: routingpb.Group.HostsEntry
	nil,                  // 9: routingpb.Host.EndpointsEntry
}
var file_pkg_proto_routingpb_routing_proto_depIdxs = []int32{
	7, // 0: routingpb.RoutingTable.groups:type_name -> routingpb.RoutingTable.GroupsEntry
	8, // 1: routingpb.Group.hosts:type_name -> routingpb.Group.HostsEntry
	6, // 2: routingpb.Host.match_rule:type_name -> routingpb.MatchRule
	0, // 3: routingpb.Host.routing_rule:type_name -> routingpb.LoadBalancer
	9, // 4: routingpb.Host.endpoints:type_name -> routingpb.Host.EndpointsEntry
	1, // 5: routingpb.MatchRule.match:type_name -> routingpb.Match
	3, // 6: routingpb.RoutingTable.GroupsEntry.value:type_name -> routingpb.Group
	4, // 7: routingpb.Group.HostsEntry.value:type_name -> routingpb.Host
	5, // 8: routingpb.Host.EndpointsEntry.value:type_name -> routingpb.Endpoint
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_pkg_proto_routingpb_routing_proto_init() }
func file_pkg_proto_routingpb_routing_proto_init() {
	if File_pkg_proto_routingpb_routing_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_proto_routingpb_routing_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RoutingTable); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_routingpb_routing_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Group); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_routingpb_routing_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Host); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_routingpb_routing_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Endpoint); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pkg_proto_routingpb_routing_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MatchRule); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pkg_proto_routingpb_routing_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pkg_proto_routingpb_routing_proto_goTypes,
		DependencyIndexes: file_pkg_proto_routingpb_routing_proto_depIdxs,
		EnumInfos:         file_pkg_proto_routingpb_routing_proto_enumTypes,
		MessageInfos:      file_pkg_proto_routingpb_routing_proto_msgTypes,
	}.Build()
	File_pkg_proto_routingpb_routing_proto = out.File
	file_pkg_proto_routingpb_routing_proto_rawDesc = nil
	file_pkg_proto_routingpb_routing_proto_goTypes = nil
	file_pkg_proto_routingpb_routing_proto_depIdxs = nil
}
