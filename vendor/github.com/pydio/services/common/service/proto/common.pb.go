// Code generated by protoc-gen-go. DO NOT EDIT.
// source: common.proto

/*
Package service is a generated protocol buffer package.

It is generated from these files:
	common.proto

It has these top-level messages:
	Query
	ActionOutputQuery
	SourceSingleQuery
*/
package service

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import google_protobuf "github.com/golang/protobuf/ptypes/any"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type OperationType int32

const (
	OperationType_OR  OperationType = 0
	OperationType_AND OperationType = 1
)

var OperationType_name = map[int32]string{
	0: "OR",
	1: "AND",
}
var OperationType_value = map[string]int32{
	"OR":  0,
	"AND": 1,
}

func (x OperationType) String() string {
	return proto.EnumName(OperationType_name, int32(x))
}
func (OperationType) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type Query struct {
	SubQueries []*google_protobuf.Any `protobuf:"bytes,1,rep,name=SubQueries" json:"SubQueries,omitempty"`
	Operation  OperationType          `protobuf:"varint,2,opt,name=Operation,enum=service.OperationType" json:"Operation,omitempty"`
	Offset     int64                  `protobuf:"varint,4,opt,name=Offset" json:"Offset,omitempty"`
	Limit      int64                  `protobuf:"varint,5,opt,name=Limit" json:"Limit,omitempty"`
	GroupBy    int32                  `protobuf:"varint,6,opt,name=groupBy" json:"groupBy,omitempty"`
}

func (m *Query) Reset()                    { *m = Query{} }
func (m *Query) String() string            { return proto.CompactTextString(m) }
func (*Query) ProtoMessage()               {}
func (*Query) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *Query) GetSubQueries() []*google_protobuf.Any {
	if m != nil {
		return m.SubQueries
	}
	return nil
}

func (m *Query) GetOperation() OperationType {
	if m != nil {
		return m.Operation
	}
	return OperationType_OR
}

func (m *Query) GetOffset() int64 {
	if m != nil {
		return m.Offset
	}
	return 0
}

func (m *Query) GetLimit() int64 {
	if m != nil {
		return m.Limit
	}
	return 0
}

func (m *Query) GetGroupBy() int32 {
	if m != nil {
		return m.GroupBy
	}
	return 0
}

type ActionOutputQuery struct {
	// Check if ActionOutput has Success = true
	Success bool `protobuf:"varint,1,opt,name=Success" json:"Success,omitempty"`
	// Check if ActionOutput has Success = false
	Failed bool `protobuf:"varint,2,opt,name=Failed" json:"Failed,omitempty"`
	// Find occurence of string in body
	StringBodyCompare string `protobuf:"bytes,3,opt,name=StringBodyCompare" json:"StringBodyCompare,omitempty"`
	// Find similar Json
	JsonBodyCompare string `protobuf:"bytes,4,opt,name=JsonBodyCompare" json:"JsonBodyCompare,omitempty"`
	// Find occurence of string in error
	ErrorStringCompare string `protobuf:"bytes,5,opt,name=ErrorStringCompare" json:"ErrorStringCompare,omitempty"`
	// Invert condition
	Not bool `protobuf:"varint,6,opt,name=Not" json:"Not,omitempty"`
}

func (m *ActionOutputQuery) Reset()                    { *m = ActionOutputQuery{} }
func (m *ActionOutputQuery) String() string            { return proto.CompactTextString(m) }
func (*ActionOutputQuery) ProtoMessage()               {}
func (*ActionOutputQuery) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *ActionOutputQuery) GetSuccess() bool {
	if m != nil {
		return m.Success
	}
	return false
}

func (m *ActionOutputQuery) GetFailed() bool {
	if m != nil {
		return m.Failed
	}
	return false
}

func (m *ActionOutputQuery) GetStringBodyCompare() string {
	if m != nil {
		return m.StringBodyCompare
	}
	return ""
}

func (m *ActionOutputQuery) GetJsonBodyCompare() string {
	if m != nil {
		return m.JsonBodyCompare
	}
	return ""
}

func (m *ActionOutputQuery) GetErrorStringCompare() string {
	if m != nil {
		return m.ErrorStringCompare
	}
	return ""
}

func (m *ActionOutputQuery) GetNot() bool {
	if m != nil {
		return m.Not
	}
	return false
}

type SourceSingleQuery struct {
	// Regexp to filter context by IP
	IPMask string `protobuf:"bytes,2,opt,name=IPMask" json:"IPMask,omitempty"`
	// Regexp to filter for a given user-agent
	UserAgent string `protobuf:"bytes,3,opt,name=UserAgent" json:"UserAgent,omitempty"`
	// Limit to a given workspaceId
	WorkspaceId string `protobuf:"bytes,4,opt,name=WorkspaceId" json:"WorkspaceId,omitempty"`
	// Invert condition
	Not bool `protobuf:"varint,5,opt,name=Not" json:"Not,omitempty"`
}

func (m *SourceSingleQuery) Reset()                    { *m = SourceSingleQuery{} }
func (m *SourceSingleQuery) String() string            { return proto.CompactTextString(m) }
func (*SourceSingleQuery) ProtoMessage()               {}
func (*SourceSingleQuery) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *SourceSingleQuery) GetIPMask() string {
	if m != nil {
		return m.IPMask
	}
	return ""
}

func (m *SourceSingleQuery) GetUserAgent() string {
	if m != nil {
		return m.UserAgent
	}
	return ""
}

func (m *SourceSingleQuery) GetWorkspaceId() string {
	if m != nil {
		return m.WorkspaceId
	}
	return ""
}

func (m *SourceSingleQuery) GetNot() bool {
	if m != nil {
		return m.Not
	}
	return false
}

func init() {
	proto.RegisterType((*Query)(nil), "service.Query")
	proto.RegisterType((*ActionOutputQuery)(nil), "service.ActionOutputQuery")
	proto.RegisterType((*SourceSingleQuery)(nil), "service.SourceSingleQuery")
	proto.RegisterEnum("service.OperationType", OperationType_name, OperationType_value)
}

func init() { proto.RegisterFile("common.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 389 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x92, 0xcb, 0x6e, 0xd3, 0x40,
	0x14, 0x86, 0x99, 0xba, 0xb6, 0xe3, 0x53, 0x2e, 0xc9, 0xa8, 0x8a, 0x46, 0x88, 0x85, 0x95, 0x95,
	0x85, 0x90, 0x17, 0xa5, 0x2f, 0xe0, 0x72, 0x91, 0x8a, 0xa0, 0x86, 0x31, 0x88, 0xb5, 0xe3, 0x9c,
	0x58, 0xa3, 0xc6, 0x33, 0xd6, 0x5c, 0x90, 0xbc, 0xe0, 0xc9, 0x78, 0x18, 0x5e, 0x05, 0xf9, 0xd6,
	0x06, 0xe8, 0x6e, 0xfe, 0xff, 0x7c, 0x3a, 0xff, 0xf9, 0x65, 0xc3, 0xe3, 0x4a, 0x35, 0x8d, 0x92,
	0x69, 0xab, 0x95, 0x55, 0x34, 0x34, 0xa8, 0x7f, 0x88, 0x0a, 0x9f, 0x47, 0xa5, 0xec, 0x46, 0x6f,
	0xf3, 0x8b, 0x80, 0xff, 0xc5, 0xa1, 0xee, 0xe8, 0x25, 0x40, 0xe1, 0xb6, 0xfd, 0x5b, 0xa0, 0x61,
	0x24, 0xf6, 0x92, 0xb3, 0x8b, 0xf3, 0xb4, 0x56, 0xaa, 0x3e, 0xe0, 0x08, 0x6f, 0xdd, 0x3e, 0xcd,
	0x64, 0xc7, 0x8f, 0x38, 0x7a, 0x09, 0x51, 0xde, 0xa2, 0x2e, 0xad, 0x50, 0x92, 0x9d, 0xc4, 0x24,
	0x79, 0x7a, 0xb1, 0x4e, 0xa7, 0x9c, 0xf4, 0x6e, 0xf2, 0xb5, 0x6b, 0x91, 0xdf, 0x83, 0x74, 0x0d,
	0x41, 0xbe, 0xdf, 0x1b, 0xb4, 0xec, 0x34, 0x26, 0x89, 0xc7, 0x27, 0x45, 0xcf, 0xc1, 0xff, 0x28,
	0x1a, 0x61, 0x99, 0x3f, 0xd8, 0xa3, 0xa0, 0x0c, 0xc2, 0x5a, 0x2b, 0xd7, 0x5e, 0x75, 0x2c, 0x88,
	0x49, 0xe2, 0xf3, 0x59, 0x6e, 0x7e, 0x13, 0x58, 0x65, 0x55, 0xbf, 0x32, 0x77, 0xb6, 0x75, 0x76,
	0x6c, 0xc2, 0x20, 0x2c, 0x5c, 0x55, 0xa1, 0xe9, 0x6b, 0x90, 0x64, 0xc1, 0x67, 0xd9, 0xe7, 0xbe,
	0x2f, 0xc5, 0x01, 0x77, 0xc3, 0xa9, 0x0b, 0x3e, 0x29, 0xfa, 0x0a, 0x56, 0x85, 0xd5, 0x42, 0xd6,
	0x57, 0x6a, 0xd7, 0xbd, 0x51, 0x4d, 0x5b, 0x6a, 0x64, 0x5e, 0x4c, 0x92, 0x88, 0xff, 0x3f, 0xa0,
	0x09, 0x3c, 0xfb, 0x60, 0x94, 0x3c, 0x66, 0x4f, 0x07, 0xf6, 0x5f, 0x9b, 0xa6, 0x40, 0xdf, 0x69,
	0xad, 0xf4, 0xb8, 0x63, 0x86, 0xfd, 0x01, 0x7e, 0x60, 0x42, 0x97, 0xe0, 0xdd, 0x28, 0x3b, 0xb4,
	0x5c, 0xf0, 0xfe, 0xb9, 0xf9, 0x09, 0xab, 0x42, 0x39, 0x5d, 0x61, 0x21, 0x64, 0x7d, 0xc0, 0xb1,
	0xe0, 0x1a, 0x82, 0xeb, 0xcf, 0x9f, 0x4a, 0x73, 0x3b, 0xd4, 0x88, 0xf8, 0xa4, 0xe8, 0x0b, 0x88,
	0xbe, 0x19, 0xd4, 0x59, 0x8d, 0xd2, 0x4e, 0xe7, 0xdf, 0x1b, 0x34, 0x86, 0xb3, 0xef, 0x4a, 0xdf,
	0x9a, 0xb6, 0xac, 0xf0, 0x7a, 0x37, 0x9d, 0x7c, 0x6c, 0xcd, 0xf1, 0xfe, 0x5d, 0xfc, 0xcb, 0x18,
	0x9e, 0xfc, 0xf5, 0x11, 0x69, 0x00, 0x27, 0x39, 0x5f, 0x3e, 0xa2, 0x21, 0x78, 0xd9, 0xcd, 0xdb,
	0x25, 0xd9, 0x06, 0xc3, 0xaf, 0xf1, 0xfa, 0x4f, 0x00, 0x00, 0x00, 0xff, 0xff, 0xe3, 0x3e, 0xef,
	0x33, 0x6b, 0x02, 0x00, 0x00,
}
