// Code generated by protoc-gen-go. DO NOT EDIT.
// source: acl.proto

/*
Package acl is a generated protocol buffer package.

It is generated from these files:
	acl.proto

It has these top-level messages:
	CreateACLRequest
	CreateACLResponse
	DeleteACLRequest
	DeleteACLResponse
	SearchACLRequest
	SearchACLResponse
	ACLAction
	ACL
	ACLSingleQuery
*/
package acl

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import service "github.com/pydio/services/common/service/proto"

import (
	client "github.com/micro/go-micro/client"
	server "github.com/micro/go-micro/server"
	context "golang.org/x/net/context"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type GroupByType int32

const (
	GroupByType_Action    GroupByType = 0
	GroupByType_Role      GroupByType = 1
	GroupByType_Workspace GroupByType = 2
	GroupByType_Node      GroupByType = 3
)

var GroupByType_name = map[int32]string{
	0: "Action",
	1: "Role",
	2: "Workspace",
	3: "Node",
}
var GroupByType_value = map[string]int32{
	"Action":    0,
	"Role":      1,
	"Workspace": 2,
	"Node":      3,
}

func (x GroupByType) String() string {
	return proto.EnumName(GroupByType_name, int32(x))
}
func (GroupByType) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type CreateACLRequest struct {
	ACL *ACL `protobuf:"bytes,1,opt,name=ACL" json:"ACL,omitempty"`
}

func (m *CreateACLRequest) Reset()                    { *m = CreateACLRequest{} }
func (m *CreateACLRequest) String() string            { return proto.CompactTextString(m) }
func (*CreateACLRequest) ProtoMessage()               {}
func (*CreateACLRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *CreateACLRequest) GetACL() *ACL {
	if m != nil {
		return m.ACL
	}
	return nil
}

type CreateACLResponse struct {
	ACL *ACL `protobuf:"bytes,1,opt,name=ACL" json:"ACL,omitempty"`
}

func (m *CreateACLResponse) Reset()                    { *m = CreateACLResponse{} }
func (m *CreateACLResponse) String() string            { return proto.CompactTextString(m) }
func (*CreateACLResponse) ProtoMessage()               {}
func (*CreateACLResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *CreateACLResponse) GetACL() *ACL {
	if m != nil {
		return m.ACL
	}
	return nil
}

type DeleteACLRequest struct {
	Query *service.Query `protobuf:"bytes,1,opt,name=Query" json:"Query,omitempty"`
}

func (m *DeleteACLRequest) Reset()                    { *m = DeleteACLRequest{} }
func (m *DeleteACLRequest) String() string            { return proto.CompactTextString(m) }
func (*DeleteACLRequest) ProtoMessage()               {}
func (*DeleteACLRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *DeleteACLRequest) GetQuery() *service.Query {
	if m != nil {
		return m.Query
	}
	return nil
}

type DeleteACLResponse struct {
	RowsDeleted int64 `protobuf:"varint,1,opt,name=RowsDeleted" json:"RowsDeleted,omitempty"`
}

func (m *DeleteACLResponse) Reset()                    { *m = DeleteACLResponse{} }
func (m *DeleteACLResponse) String() string            { return proto.CompactTextString(m) }
func (*DeleteACLResponse) ProtoMessage()               {}
func (*DeleteACLResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *DeleteACLResponse) GetRowsDeleted() int64 {
	if m != nil {
		return m.RowsDeleted
	}
	return 0
}

type SearchACLRequest struct {
	Query *service.Query `protobuf:"bytes,1,opt,name=Query" json:"Query,omitempty"`
}

func (m *SearchACLRequest) Reset()                    { *m = SearchACLRequest{} }
func (m *SearchACLRequest) String() string            { return proto.CompactTextString(m) }
func (*SearchACLRequest) ProtoMessage()               {}
func (*SearchACLRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *SearchACLRequest) GetQuery() *service.Query {
	if m != nil {
		return m.Query
	}
	return nil
}

type SearchACLResponse struct {
	ACL *ACL `protobuf:"bytes,1,opt,name=ACL" json:"ACL,omitempty"`
}

func (m *SearchACLResponse) Reset()                    { *m = SearchACLResponse{} }
func (m *SearchACLResponse) String() string            { return proto.CompactTextString(m) }
func (*SearchACLResponse) ProtoMessage()               {}
func (*SearchACLResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *SearchACLResponse) GetACL() *ACL {
	if m != nil {
		return m.ACL
	}
	return nil
}

type ACLAction struct {
	Name  string `protobuf:"bytes,1,opt,name=Name" json:"Name,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=Value" json:"Value,omitempty"`
}

func (m *ACLAction) Reset()                    { *m = ACLAction{} }
func (m *ACLAction) String() string            { return proto.CompactTextString(m) }
func (*ACLAction) ProtoMessage()               {}
func (*ACLAction) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

func (m *ACLAction) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *ACLAction) GetValue() string {
	if m != nil {
		return m.Value
	}
	return ""
}

type ACL struct {
	ID          string     `protobuf:"bytes,1,opt,name=ID" json:"ID,omitempty"`
	Action      *ACLAction `protobuf:"bytes,2,opt,name=Action" json:"Action,omitempty"`
	RoleID      string     `protobuf:"bytes,3,opt,name=RoleID" json:"RoleID,omitempty"`
	WorkspaceID string     `protobuf:"bytes,4,opt,name=WorkspaceID" json:"WorkspaceID,omitempty"`
	NodeID      string     `protobuf:"bytes,5,opt,name=NodeID" json:"NodeID,omitempty"`
}

func (m *ACL) Reset()                    { *m = ACL{} }
func (m *ACL) String() string            { return proto.CompactTextString(m) }
func (*ACL) ProtoMessage()               {}
func (*ACL) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

func (m *ACL) GetID() string {
	if m != nil {
		return m.ID
	}
	return ""
}

func (m *ACL) GetAction() *ACLAction {
	if m != nil {
		return m.Action
	}
	return nil
}

func (m *ACL) GetRoleID() string {
	if m != nil {
		return m.RoleID
	}
	return ""
}

func (m *ACL) GetWorkspaceID() string {
	if m != nil {
		return m.WorkspaceID
	}
	return ""
}

func (m *ACL) GetNodeID() string {
	if m != nil {
		return m.NodeID
	}
	return ""
}

type ACLSingleQuery struct {
	Actions      []*ACLAction `protobuf:"bytes,1,rep,name=Actions" json:"Actions,omitempty"`
	RoleIDs      []string     `protobuf:"bytes,2,rep,name=RoleIDs" json:"RoleIDs,omitempty"`
	WorkspaceIDs []string     `protobuf:"bytes,3,rep,name=WorkspaceIDs" json:"WorkspaceIDs,omitempty"`
	NodeIDs      []string     `protobuf:"bytes,4,rep,name=NodeIDs" json:"NodeIDs,omitempty"`
	Not          bool         `protobuf:"varint,5,opt,name=not" json:"not,omitempty"`
}

func (m *ACLSingleQuery) Reset()                    { *m = ACLSingleQuery{} }
func (m *ACLSingleQuery) String() string            { return proto.CompactTextString(m) }
func (*ACLSingleQuery) ProtoMessage()               {}
func (*ACLSingleQuery) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{8} }

func (m *ACLSingleQuery) GetActions() []*ACLAction {
	if m != nil {
		return m.Actions
	}
	return nil
}

func (m *ACLSingleQuery) GetRoleIDs() []string {
	if m != nil {
		return m.RoleIDs
	}
	return nil
}

func (m *ACLSingleQuery) GetWorkspaceIDs() []string {
	if m != nil {
		return m.WorkspaceIDs
	}
	return nil
}

func (m *ACLSingleQuery) GetNodeIDs() []string {
	if m != nil {
		return m.NodeIDs
	}
	return nil
}

func (m *ACLSingleQuery) GetNot() bool {
	if m != nil {
		return m.Not
	}
	return false
}

func init() {
	proto.RegisterType((*CreateACLRequest)(nil), "acl.CreateACLRequest")
	proto.RegisterType((*CreateACLResponse)(nil), "acl.CreateACLResponse")
	proto.RegisterType((*DeleteACLRequest)(nil), "acl.DeleteACLRequest")
	proto.RegisterType((*DeleteACLResponse)(nil), "acl.DeleteACLResponse")
	proto.RegisterType((*SearchACLRequest)(nil), "acl.SearchACLRequest")
	proto.RegisterType((*SearchACLResponse)(nil), "acl.SearchACLResponse")
	proto.RegisterType((*ACLAction)(nil), "acl.ACLAction")
	proto.RegisterType((*ACL)(nil), "acl.ACL")
	proto.RegisterType((*ACLSingleQuery)(nil), "acl.ACLSingleQuery")
	proto.RegisterEnum("acl.GroupByType", GroupByType_name, GroupByType_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ client.Option
var _ server.Option

// Client API for ACLService service

type ACLServiceClient interface {
	CreateACL(ctx context.Context, in *CreateACLRequest, opts ...client.CallOption) (*CreateACLResponse, error)
	DeleteACL(ctx context.Context, in *DeleteACLRequest, opts ...client.CallOption) (*DeleteACLResponse, error)
	SearchACL(ctx context.Context, in *SearchACLRequest, opts ...client.CallOption) (ACLService_SearchACLClient, error)
	StreamACL(ctx context.Context, opts ...client.CallOption) (ACLService_StreamACLClient, error)
}

type aCLServiceClient struct {
	c           client.Client
	serviceName string
}

func NewACLServiceClient(serviceName string, c client.Client) ACLServiceClient {
	if c == nil {
		c = client.NewClient()
	}
	if len(serviceName) == 0 {
		serviceName = "acl"
	}
	return &aCLServiceClient{
		c:           c,
		serviceName: serviceName,
	}
}

func (c *aCLServiceClient) CreateACL(ctx context.Context, in *CreateACLRequest, opts ...client.CallOption) (*CreateACLResponse, error) {
	req := c.c.NewRequest(c.serviceName, "ACLService.CreateACL", in)
	out := new(CreateACLResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *aCLServiceClient) DeleteACL(ctx context.Context, in *DeleteACLRequest, opts ...client.CallOption) (*DeleteACLResponse, error) {
	req := c.c.NewRequest(c.serviceName, "ACLService.DeleteACL", in)
	out := new(DeleteACLResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *aCLServiceClient) SearchACL(ctx context.Context, in *SearchACLRequest, opts ...client.CallOption) (ACLService_SearchACLClient, error) {
	req := c.c.NewRequest(c.serviceName, "ACLService.SearchACL", &SearchACLRequest{})
	stream, err := c.c.Stream(ctx, req, opts...)
	if err != nil {
		return nil, err
	}
	if err := stream.Send(in); err != nil {
		return nil, err
	}
	return &aCLServiceSearchACLClient{stream}, nil
}

type ACLService_SearchACLClient interface {
	SendMsg(interface{}) error
	RecvMsg(interface{}) error
	Close() error
	Recv() (*SearchACLResponse, error)
}

type aCLServiceSearchACLClient struct {
	stream client.Streamer
}

func (x *aCLServiceSearchACLClient) Close() error {
	return x.stream.Close()
}

func (x *aCLServiceSearchACLClient) SendMsg(m interface{}) error {
	return x.stream.Send(m)
}

func (x *aCLServiceSearchACLClient) RecvMsg(m interface{}) error {
	return x.stream.Recv(m)
}

func (x *aCLServiceSearchACLClient) Recv() (*SearchACLResponse, error) {
	m := new(SearchACLResponse)
	err := x.stream.Recv(m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (c *aCLServiceClient) StreamACL(ctx context.Context, opts ...client.CallOption) (ACLService_StreamACLClient, error) {
	req := c.c.NewRequest(c.serviceName, "ACLService.StreamACL", &SearchACLRequest{})
	stream, err := c.c.Stream(ctx, req, opts...)
	if err != nil {
		return nil, err
	}
	return &aCLServiceStreamACLClient{stream}, nil
}

type ACLService_StreamACLClient interface {
	SendMsg(interface{}) error
	RecvMsg(interface{}) error
	Close() error
	Send(*SearchACLRequest) error
	Recv() (*SearchACLResponse, error)
}

type aCLServiceStreamACLClient struct {
	stream client.Streamer
}

func (x *aCLServiceStreamACLClient) Close() error {
	return x.stream.Close()
}

func (x *aCLServiceStreamACLClient) SendMsg(m interface{}) error {
	return x.stream.Send(m)
}

func (x *aCLServiceStreamACLClient) RecvMsg(m interface{}) error {
	return x.stream.Recv(m)
}

func (x *aCLServiceStreamACLClient) Send(m *SearchACLRequest) error {
	return x.stream.Send(m)
}

func (x *aCLServiceStreamACLClient) Recv() (*SearchACLResponse, error) {
	m := new(SearchACLResponse)
	err := x.stream.Recv(m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for ACLService service

type ACLServiceHandler interface {
	CreateACL(context.Context, *CreateACLRequest, *CreateACLResponse) error
	DeleteACL(context.Context, *DeleteACLRequest, *DeleteACLResponse) error
	SearchACL(context.Context, *SearchACLRequest, ACLService_SearchACLStream) error
	StreamACL(context.Context, ACLService_StreamACLStream) error
}

func RegisterACLServiceHandler(s server.Server, hdlr ACLServiceHandler, opts ...server.HandlerOption) {
	s.Handle(s.NewHandler(&ACLService{hdlr}, opts...))
}

type ACLService struct {
	ACLServiceHandler
}

func (h *ACLService) CreateACL(ctx context.Context, in *CreateACLRequest, out *CreateACLResponse) error {
	return h.ACLServiceHandler.CreateACL(ctx, in, out)
}

func (h *ACLService) DeleteACL(ctx context.Context, in *DeleteACLRequest, out *DeleteACLResponse) error {
	return h.ACLServiceHandler.DeleteACL(ctx, in, out)
}

func (h *ACLService) SearchACL(ctx context.Context, stream server.Streamer) error {
	m := new(SearchACLRequest)
	if err := stream.Recv(m); err != nil {
		return err
	}
	return h.ACLServiceHandler.SearchACL(ctx, m, &aCLServiceSearchACLStream{stream})
}

type ACLService_SearchACLStream interface {
	SendMsg(interface{}) error
	RecvMsg(interface{}) error
	Close() error
	Send(*SearchACLResponse) error
}

type aCLServiceSearchACLStream struct {
	stream server.Streamer
}

func (x *aCLServiceSearchACLStream) Close() error {
	return x.stream.Close()
}

func (x *aCLServiceSearchACLStream) SendMsg(m interface{}) error {
	return x.stream.Send(m)
}

func (x *aCLServiceSearchACLStream) RecvMsg(m interface{}) error {
	return x.stream.Recv(m)
}

func (x *aCLServiceSearchACLStream) Send(m *SearchACLResponse) error {
	return x.stream.Send(m)
}

func (h *ACLService) StreamACL(ctx context.Context, stream server.Streamer) error {
	return h.ACLServiceHandler.StreamACL(ctx, &aCLServiceStreamACLStream{stream})
}

type ACLService_StreamACLStream interface {
	SendMsg(interface{}) error
	RecvMsg(interface{}) error
	Close() error
	Send(*SearchACLResponse) error
	Recv() (*SearchACLRequest, error)
}

type aCLServiceStreamACLStream struct {
	stream server.Streamer
}

func (x *aCLServiceStreamACLStream) Close() error {
	return x.stream.Close()
}

func (x *aCLServiceStreamACLStream) SendMsg(m interface{}) error {
	return x.stream.Send(m)
}

func (x *aCLServiceStreamACLStream) RecvMsg(m interface{}) error {
	return x.stream.Recv(m)
}

func (x *aCLServiceStreamACLStream) Send(m *SearchACLResponse) error {
	return x.stream.Send(m)
}

func (x *aCLServiceStreamACLStream) Recv() (*SearchACLRequest, error) {
	m := new(SearchACLRequest)
	if err := x.stream.Recv(m); err != nil {
		return nil, err
	}
	return m, nil
}

func init() { proto.RegisterFile("acl.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 502 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x53, 0x5d, 0x8b, 0xd3, 0x40,
	0x14, 0x6d, 0x92, 0xb6, 0xdb, 0xdc, 0x6a, 0x49, 0x2f, 0xba, 0x84, 0x3e, 0x95, 0x41, 0xa4, 0xf8,
	0x90, 0x2c, 0x2b, 0x0b, 0x82, 0x8b, 0x18, 0x53, 0x90, 0x42, 0x59, 0x70, 0x2a, 0xfa, 0x9c, 0x4d,
	0x87, 0xdd, 0x60, 0x92, 0x89, 0x99, 0x44, 0xe9, 0x5f, 0xf0, 0xc5, 0x3f, 0xe1, 0x0f, 0x95, 0xf9,
	0x68, 0xc9, 0x76, 0x45, 0x74, 0xdf, 0x72, 0xcf, 0xb9, 0xe7, 0x9e, 0x3b, 0x93, 0x33, 0xe0, 0x26,
	0x69, 0x1e, 0x54, 0x35, 0x6f, 0x38, 0x3a, 0x49, 0x9a, 0xcf, 0x5e, 0xdf, 0x64, 0xcd, 0x6d, 0x7b,
	0x1d, 0xa4, 0xbc, 0x08, 0xab, 0xdd, 0x36, 0xe3, 0xa1, 0x60, 0xf5, 0xb7, 0x2c, 0x65, 0x22, 0x4c,
	0x79, 0x51, 0xf0, 0x72, 0x5f, 0x87, 0x4a, 0x64, 0x40, 0x3d, 0x81, 0x04, 0xe0, 0xc5, 0x35, 0x4b,
	0x1a, 0x16, 0xc5, 0x6b, 0xca, 0xbe, 0xb6, 0x4c, 0x34, 0x38, 0x03, 0x27, 0x8a, 0xd7, 0xbe, 0x35,
	0xb7, 0x16, 0xe3, 0xf3, 0x51, 0x20, 0xed, 0x24, 0x2b, 0x41, 0x12, 0xc2, 0xb4, 0xd3, 0x2f, 0x2a,
	0x5e, 0x0a, 0xf6, 0x57, 0xc1, 0x2b, 0xf0, 0x96, 0x2c, 0x67, 0x77, 0x0c, 0x9e, 0xc1, 0xe0, 0x43,
	0xcb, 0xea, 0x9d, 0x51, 0x4c, 0x02, 0xb3, 0x60, 0xa0, 0x50, 0xaa, 0x49, 0x72, 0x01, 0xd3, 0x8e,
	0xd2, 0x58, 0xcd, 0x61, 0x4c, 0xf9, 0x77, 0xa1, 0x89, 0xad, 0x1a, 0xe0, 0xd0, 0x2e, 0x24, 0x0d,
	0x37, 0x2c, 0xa9, 0xd3, 0xdb, 0xff, 0x36, 0x0c, 0x61, 0xda, 0x51, 0xfe, 0xc3, 0xd9, 0x2e, 0xc0,
	0x8d, 0xe2, 0x75, 0x94, 0x36, 0x19, 0x2f, 0x11, 0xa1, 0x7f, 0x95, 0x14, 0x4c, 0x75, 0xba, 0x54,
	0x7d, 0xe3, 0x13, 0x18, 0x7c, 0x4a, 0xf2, 0x96, 0xf9, 0xb6, 0x02, 0x75, 0x41, 0x7e, 0x5a, 0x6a,
	0x26, 0x4e, 0xc0, 0x5e, 0x2d, 0x4d, 0xbf, 0xbd, 0x5a, 0xe2, 0x73, 0x18, 0xea, 0x59, 0xaa, 0x5d,
	0xae, 0x69, 0xdc, 0x34, 0x4a, 0x0d, 0x8b, 0xa7, 0x30, 0xa4, 0x3c, 0x67, 0xab, 0xa5, 0xef, 0x28,
	0xad, 0xa9, 0xe4, 0xdd, 0x7c, 0xe6, 0xf5, 0x17, 0x51, 0x25, 0xa9, 0x24, 0xfb, 0x8a, 0xec, 0x42,
	0x52, 0x79, 0xc5, 0xb7, 0x92, 0x1c, 0x68, 0xa5, 0xae, 0xc8, 0x2f, 0x0b, 0x26, 0x51, 0xbc, 0xde,
	0x64, 0xe5, 0x4d, 0xce, 0xd4, 0x65, 0xe0, 0x02, 0x4e, 0xb4, 0x9d, 0xf0, 0xad, 0xb9, 0xf3, 0x87,
	0x6d, 0xf6, 0x34, 0xfa, 0x70, 0xa2, 0x17, 0x10, 0xbe, 0x3d, 0x77, 0x16, 0x2e, 0xdd, 0x97, 0x48,
	0xe0, 0x51, 0xc7, 0x5d, 0xf8, 0x8e, 0xa2, 0xef, 0x60, 0x52, 0xad, 0x97, 0x10, 0x7e, 0x5f, 0xab,
	0x4d, 0x89, 0x1e, 0x38, 0x25, 0x6f, 0xd4, 0xa6, 0x23, 0x2a, 0x3f, 0x5f, 0x5c, 0xc2, 0xf8, 0x7d,
	0xcd, 0xdb, 0xea, 0xdd, 0xee, 0xe3, 0xae, 0x62, 0x08, 0xfb, 0xfb, 0xf2, 0x7a, 0x38, 0x82, 0xbe,
	0x74, 0xf5, 0x2c, 0x7c, 0x0c, 0xee, 0xc1, 0xc0, 0xb3, 0x25, 0x21, 0x07, 0x7a, 0xce, 0xf9, 0x0f,
	0x1b, 0x40, 0x1e, 0x52, 0xff, 0x7a, 0xbc, 0x04, 0xf7, 0x90, 0x64, 0x7c, 0xaa, 0x0e, 0x77, 0xfc,
	0x12, 0x66, 0xa7, 0xc7, 0xb0, 0x0e, 0x05, 0xe9, 0x49, 0xf5, 0x21, 0x9c, 0x46, 0x7d, 0x1c, 0x73,
	0xa3, 0xbe, 0x97, 0x61, 0xd2, 0xc3, 0x37, 0xe0, 0x1e, 0x92, 0x66, 0xd4, 0xc7, 0x99, 0x35, 0xea,
	0x7b, 0x81, 0x24, 0xbd, 0x33, 0x0b, 0xdf, 0x82, 0xbb, 0x69, 0x6a, 0x96, 0x14, 0x0f, 0xd1, 0x2f,
	0xac, 0x33, 0xeb, 0x7a, 0xa8, 0x9e, 0xff, 0xcb, 0xdf, 0x01, 0x00, 0x00, 0xff, 0xff, 0x83, 0xbd,
	0xa9, 0x74, 0x4d, 0x04, 0x00, 0x00,
}
