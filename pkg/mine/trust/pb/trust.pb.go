// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: trust.proto

package pb

import (
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type TrustSign struct {
	Id                   int32    `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Op                   int32    `protobuf:"varint,2,opt,name=op,proto3" json:"op,omitempty"`
	Peer                 []byte   `protobuf:"bytes,3,opt,name=Peer,proto3" json:"Peer,omitempty"`
	Data                 []byte   `protobuf:"bytes,4,opt,name=data,proto3" json:"data,omitempty"`
	Expire               int64    `protobuf:"varint,5,opt,name=expire,proto3" json:"expire,omitempty"`
	Result               bool     `protobuf:"varint,6,opt,name=result,proto3" json:"result,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *TrustSign) Reset()         { *m = TrustSign{} }
func (m *TrustSign) String() string { return proto.CompactTextString(m) }
func (*TrustSign) ProtoMessage()    {}
func (*TrustSign) Descriptor() ([]byte, []int) {
	return fileDescriptor_05bd52222ab7be52, []int{0}
}
func (m *TrustSign) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TrustSign.Unmarshal(m, b)
}
func (m *TrustSign) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TrustSign.Marshal(b, m, deterministic)
}
func (m *TrustSign) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TrustSign.Merge(m, src)
}
func (m *TrustSign) XXX_Size() int {
	return xxx_messageInfo_TrustSign.Size(m)
}
func (m *TrustSign) XXX_DiscardUnknown() {
	xxx_messageInfo_TrustSign.DiscardUnknown(m)
}

var xxx_messageInfo_TrustSign proto.InternalMessageInfo

func (m *TrustSign) GetId() int32 {
	if m != nil {
		return m.Id
	}
	return 0
}

func (m *TrustSign) GetOp() int32 {
	if m != nil {
		return m.Op
	}
	return 0
}

func (m *TrustSign) GetPeer() []byte {
	if m != nil {
		return m.Peer
	}
	return nil
}

func (m *TrustSign) GetData() []byte {
	if m != nil {
		return m.Data
	}
	return nil
}

func (m *TrustSign) GetExpire() int64 {
	if m != nil {
		return m.Expire
	}
	return 0
}

func (m *TrustSign) GetResult() bool {
	if m != nil {
		return m.Result
	}
	return false
}

type Trust struct {
	Expire               int64    `protobuf:"varint,1,opt,name=expire,proto3" json:"expire,omitempty"`
	Stream               []byte   `protobuf:"bytes,2,opt,name=stream,proto3" json:"stream,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Trust) Reset()         { *m = Trust{} }
func (m *Trust) String() string { return proto.CompactTextString(m) }
func (*Trust) ProtoMessage()    {}
func (*Trust) Descriptor() ([]byte, []int) {
	return fileDescriptor_05bd52222ab7be52, []int{1}
}
func (m *Trust) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Trust.Unmarshal(m, b)
}
func (m *Trust) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Trust.Marshal(b, m, deterministic)
}
func (m *Trust) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Trust.Merge(m, src)
}
func (m *Trust) XXX_Size() int {
	return xxx_messageInfo_Trust.Size(m)
}
func (m *Trust) XXX_DiscardUnknown() {
	xxx_messageInfo_Trust.DiscardUnknown(m)
}

var xxx_messageInfo_Trust proto.InternalMessageInfo

func (m *Trust) GetExpire() int64 {
	if m != nil {
		return m.Expire
	}
	return 0
}

func (m *Trust) GetStream() []byte {
	if m != nil {
		return m.Stream
	}
	return nil
}

func init() {
	proto.RegisterType((*TrustSign)(nil), "trust.TrustSign")
	proto.RegisterType((*Trust)(nil), "trust.Trust")
}

func init() { proto.RegisterFile("trust.proto", fileDescriptor_05bd52222ab7be52) }

var fileDescriptor_05bd52222ab7be52 = []byte{
	// 175 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x5c, 0xcf, 0x31, 0x0b, 0xc2, 0x30,
	0x10, 0x05, 0x60, 0x2e, 0x6d, 0x8a, 0xc6, 0xe2, 0x90, 0x41, 0x32, 0x86, 0x4e, 0x99, 0x5c, 0x1c,
	0xdc, 0xfd, 0x05, 0x12, 0x9d, 0xdc, 0x5a, 0x1a, 0x24, 0xa0, 0x26, 0x5c, 0xaf, 0xe0, 0xee, 0x1f,
	0x97, 0xa4, 0x19, 0xc4, 0xed, 0x7d, 0x0f, 0x0e, 0xde, 0x89, 0x0d, 0xe1, 0x3c, 0xd1, 0x3e, 0x62,
	0xa0, 0x20, 0x79, 0x46, 0xf7, 0x01, 0xb1, 0xbe, 0xa6, 0x74, 0xf1, 0xf7, 0x97, 0xdc, 0x0a, 0xe6,
	0x47, 0x05, 0x1a, 0x0c, 0xb7, 0xcc, 0x8f, 0xc9, 0x21, 0x2a, 0xb6, 0x38, 0x44, 0x29, 0x45, 0x7d,
	0x76, 0x0e, 0x55, 0xa5, 0xc1, 0xb4, 0x36, 0xe7, 0xd4, 0x8d, 0x3d, 0xf5, 0xaa, 0x5e, 0xba, 0x94,
	0xe5, 0x4e, 0x34, 0xee, 0x1d, 0x3d, 0x3a, 0xc5, 0x35, 0x98, 0xca, 0x16, 0xa5, 0x1e, 0xdd, 0x34,
	0x3f, 0x48, 0x35, 0x1a, 0xcc, 0xca, 0x16, 0x75, 0x47, 0xc1, 0xf3, 0x88, 0x9f, 0x43, 0xf8, 0x3f,
	0x9c, 0x08, 0x5d, 0xff, 0xcc, 0x63, 0x5a, 0x5b, 0x74, 0xaa, 0x6f, 0x2c, 0x0e, 0x43, 0x93, 0x5f,
	0x3a, 0x7c, 0x03, 0x00, 0x00, 0xff, 0xff, 0x17, 0xbe, 0x48, 0xb9, 0xe1, 0x00, 0x00, 0x00,
}
