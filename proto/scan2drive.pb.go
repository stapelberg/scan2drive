// Code generated by protoc-gen-go.
// source: scan2drive.proto
// DO NOT EDIT!

/*
Package proto is a generated protocol buffer package.

It is generated from these files:
	scan2drive.proto

It has these top-level messages:
	DefaultUserRequest
	DefaultUserReply
	ProcessScanRequest
	ProcessScanReply
	ConvertRequest
	ConvertReply
*/
package proto

import proto1 "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto1.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
const _ = proto1.ProtoPackageIsVersion1

type DefaultUserRequest struct {
}

func (m *DefaultUserRequest) Reset()                    { *m = DefaultUserRequest{} }
func (m *DefaultUserRequest) String() string            { return proto1.CompactTextString(m) }
func (*DefaultUserRequest) ProtoMessage()               {}
func (*DefaultUserRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type DefaultUserReply struct {
	User string `protobuf:"bytes,1,opt,name=user" json:"user,omitempty"`
}

func (m *DefaultUserReply) Reset()                    { *m = DefaultUserReply{} }
func (m *DefaultUserReply) String() string            { return proto1.CompactTextString(m) }
func (*DefaultUserReply) ProtoMessage()               {}
func (*DefaultUserReply) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

type ProcessScanRequest struct {
	User string `protobuf:"bytes,1,opt,name=user" json:"user,omitempty"`
	Dir  string `protobuf:"bytes,2,opt,name=dir" json:"dir,omitempty"`
}

func (m *ProcessScanRequest) Reset()                    { *m = ProcessScanRequest{} }
func (m *ProcessScanRequest) String() string            { return proto1.CompactTextString(m) }
func (*ProcessScanRequest) ProtoMessage()               {}
func (*ProcessScanRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

type ProcessScanReply struct {
}

func (m *ProcessScanReply) Reset()                    { *m = ProcessScanReply{} }
func (m *ProcessScanReply) String() string            { return proto1.CompactTextString(m) }
func (*ProcessScanReply) ProtoMessage()               {}
func (*ProcessScanReply) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

type ConvertRequest struct {
	ScannedPage [][]byte `protobuf:"bytes,1,rep,name=scanned_page,json=scannedPage,proto3" json:"scanned_page,omitempty"`
}

func (m *ConvertRequest) Reset()                    { *m = ConvertRequest{} }
func (m *ConvertRequest) String() string            { return proto1.CompactTextString(m) }
func (*ConvertRequest) ProtoMessage()               {}
func (*ConvertRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

type ConvertReply struct {
	PDF []byte `protobuf:"bytes,1,opt,name=PDF,json=pDF,proto3" json:"PDF,omitempty"`
	// A thumbnail (PNG).
	Thumb []byte `protobuf:"bytes,2,opt,name=thumb,proto3" json:"thumb,omitempty"`
}

func (m *ConvertReply) Reset()                    { *m = ConvertReply{} }
func (m *ConvertReply) String() string            { return proto1.CompactTextString(m) }
func (*ConvertReply) ProtoMessage()               {}
func (*ConvertReply) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func init() {
	proto1.RegisterType((*DefaultUserRequest)(nil), "proto.DefaultUserRequest")
	proto1.RegisterType((*DefaultUserReply)(nil), "proto.DefaultUserReply")
	proto1.RegisterType((*ProcessScanRequest)(nil), "proto.ProcessScanRequest")
	proto1.RegisterType((*ProcessScanReply)(nil), "proto.ProcessScanReply")
	proto1.RegisterType((*ConvertRequest)(nil), "proto.ConvertRequest")
	proto1.RegisterType((*ConvertReply)(nil), "proto.ConvertReply")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// Client API for Scan service

type ScanClient interface {
	DefaultUser(ctx context.Context, in *DefaultUserRequest, opts ...grpc.CallOption) (*DefaultUserReply, error)
	ProcessScan(ctx context.Context, in *ProcessScanRequest, opts ...grpc.CallOption) (*ProcessScanReply, error)
	Convert(ctx context.Context, in *ConvertRequest, opts ...grpc.CallOption) (*ConvertReply, error)
}

type scanClient struct {
	cc *grpc.ClientConn
}

func NewScanClient(cc *grpc.ClientConn) ScanClient {
	return &scanClient{cc}
}

func (c *scanClient) DefaultUser(ctx context.Context, in *DefaultUserRequest, opts ...grpc.CallOption) (*DefaultUserReply, error) {
	out := new(DefaultUserReply)
	err := grpc.Invoke(ctx, "/proto.Scan/DefaultUser", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scanClient) ProcessScan(ctx context.Context, in *ProcessScanRequest, opts ...grpc.CallOption) (*ProcessScanReply, error) {
	out := new(ProcessScanReply)
	err := grpc.Invoke(ctx, "/proto.Scan/ProcessScan", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *scanClient) Convert(ctx context.Context, in *ConvertRequest, opts ...grpc.CallOption) (*ConvertReply, error) {
	out := new(ConvertReply)
	err := grpc.Invoke(ctx, "/proto.Scan/Convert", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Scan service

type ScanServer interface {
	DefaultUser(context.Context, *DefaultUserRequest) (*DefaultUserReply, error)
	ProcessScan(context.Context, *ProcessScanRequest) (*ProcessScanReply, error)
	Convert(context.Context, *ConvertRequest) (*ConvertReply, error)
}

func RegisterScanServer(s *grpc.Server, srv ScanServer) {
	s.RegisterService(&_Scan_serviceDesc, srv)
}

func _Scan_DefaultUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(DefaultUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	out, err := srv.(ScanServer).DefaultUser(ctx, in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func _Scan_ProcessScan_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(ProcessScanRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	out, err := srv.(ScanServer).ProcessScan(ctx, in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func _Scan_Convert_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error) (interface{}, error) {
	in := new(ConvertRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	out, err := srv.(ScanServer).Convert(ctx, in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

var _Scan_serviceDesc = grpc.ServiceDesc{
	ServiceName: "proto.Scan",
	HandlerType: (*ScanServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "DefaultUser",
			Handler:    _Scan_DefaultUser_Handler,
		},
		{
			MethodName: "ProcessScan",
			Handler:    _Scan_ProcessScan_Handler,
		},
		{
			MethodName: "Convert",
			Handler:    _Scan_Convert_Handler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

var fileDescriptor0 = []byte{
	// 270 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x6c, 0x50, 0xc1, 0x4e, 0xc2, 0x40,
	0x10, 0xa5, 0x16, 0x34, 0x0e, 0x1b, 0xd3, 0x8c, 0x18, 0x91, 0x93, 0xee, 0xc1, 0x70, 0xe2, 0x00,
	0x89, 0x26, 0x5e, 0x69, 0x38, 0x37, 0x6b, 0x3c, 0x9b, 0x42, 0x47, 0x25, 0xc1, 0xb6, 0xee, 0x6e,
	0x49, 0xfc, 0x44, 0xff, 0xca, 0xdd, 0x65, 0x6d, 0xa8, 0xed, 0xa9, 0x3b, 0xaf, 0xf3, 0xde, 0x9b,
	0xf7, 0x20, 0x52, 0x9b, 0x34, 0x9f, 0x67, 0x72, 0xbb, 0xa7, 0x59, 0x29, 0x0b, 0x5d, 0xe0, 0xc0,
	0x7d, 0xf8, 0x08, 0x30, 0xa6, 0xb7, 0xb4, 0xda, 0xe9, 0x17, 0x45, 0x52, 0xd0, 0x57, 0x45, 0x4a,
	0xf3, 0x7b, 0x88, 0x1a, 0x68, 0xb9, 0xfb, 0x46, 0x84, 0x7e, 0x65, 0x86, 0x71, 0x70, 0x1b, 0x4c,
	0xcf, 0x85, 0x7b, 0xf3, 0x27, 0xc0, 0x44, 0x16, 0x1b, 0x52, 0xea, 0xd9, 0xe8, 0x7b, 0x76, 0xd7,
	0x26, 0x46, 0x10, 0x66, 0x5b, 0x39, 0x3e, 0x71, 0x90, 0x7d, 0x72, 0x84, 0xa8, 0xc1, 0x35, 0x1e,
	0x7c, 0x01, 0x17, 0xcb, 0x22, 0xdf, 0x93, 0xd4, 0x7f, 0x5a, 0x77, 0xc0, 0xec, 0xe9, 0x39, 0x65,
	0xaf, 0x65, 0xfa, 0x4e, 0x46, 0x33, 0x9c, 0x32, 0x31, 0xf4, 0x58, 0x62, 0x20, 0xfe, 0x00, 0xac,
	0x26, 0xd9, 0x43, 0x8d, 0x55, 0x12, 0xaf, 0x9c, 0x3b, 0x13, 0x61, 0x19, 0xaf, 0x70, 0x04, 0x03,
	0xfd, 0x51, 0x7d, 0xae, 0x9d, 0x3d, 0x13, 0x87, 0x61, 0xfe, 0x13, 0x40, 0xdf, 0x5a, 0xe3, 0x12,
	0x86, 0x47, 0x69, 0xf1, 0xe6, 0xd0, 0xd0, 0xac, 0xdd, 0xcb, 0xe4, 0xba, 0xeb, 0x97, 0x3d, 0xbc,
	0x67, 0x45, 0x8e, 0xe2, 0xd4, 0x22, 0xed, 0x7a, 0x6a, 0x91, 0x56, 0xfa, 0x1e, 0x3e, 0xc2, 0x99,
	0x8f, 0x82, 0x57, 0x7e, 0xab, 0xd9, 0xc7, 0xe4, 0xf2, 0x3f, 0xec, 0x88, 0xeb, 0x53, 0x87, 0x2e,
	0x7e, 0x03, 0x00, 0x00, 0xff, 0xff, 0x2c, 0x06, 0x41, 0xfd, 0xe8, 0x01, 0x00, 0x00,
}
