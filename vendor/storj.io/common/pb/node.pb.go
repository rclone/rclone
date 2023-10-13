// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: node.proto

package pb

import (
	fmt "fmt"
	math "math"
	time "time"

	proto "github.com/gogo/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf
var _ = time.Kitchen

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// NodeType is an enum of possible node types.
type NodeType int32

const (
	NodeType_INVALID   NodeType = 0
	NodeType_SATELLITE NodeType = 1
	NodeType_STORAGE   NodeType = 2
	NodeType_UPLINK    NodeType = 3
	NodeType_BOOTSTRAP NodeType = 4 // Deprecated: Do not use.
)

var NodeType_name = map[int32]string{
	0: "INVALID",
	1: "SATELLITE",
	2: "STORAGE",
	3: "UPLINK",
	4: "BOOTSTRAP",
}

var NodeType_value = map[string]int32{
	"INVALID":   0,
	"SATELLITE": 1,
	"STORAGE":   2,
	"UPLINK":    3,
	"BOOTSTRAP": 4,
}

func (x NodeType) String() string {
	return proto.EnumName(NodeType_name, int32(x))
}

func (NodeType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{0}
}

// NodeTransport is an enum of possible transports for the overlay network.
type NodeTransport int32

const (
	NodeTransport_TCP_TLS_RPC   NodeTransport = 0
	NodeTransport_QUIC_RPC      NodeTransport = 1
	NodeTransport_TCP_NOISE_RPC NodeTransport = 2
)

var NodeTransport_name = map[int32]string{
	0: "TCP_TLS_RPC",
	1: "QUIC_RPC",
	2: "TCP_NOISE_RPC",
}

var NodeTransport_value = map[string]int32{
	"TCP_TLS_RPC":   0,
	"QUIC_RPC":      1,
	"TCP_NOISE_RPC": 2,
}

func (x NodeTransport) String() string {
	return proto.EnumName(NodeTransport_name, int32(x))
}

func (NodeTransport) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{1}
}

type NodeAddress_Feature int32

const (
	NodeAddress_NO_FEATURES NodeAddress_Feature = 0
	// these must all be powers of two, since they are bitwise ORed into
	// the features field.
	NodeAddress_TCP_FASTOPEN_ENABLED NodeAddress_Feature = 1
)

var NodeAddress_Feature_name = map[int32]string{
	0: "NO_FEATURES",
	1: "TCP_FASTOPEN_ENABLED",
}

var NodeAddress_Feature_value = map[string]int32{
	"NO_FEATURES":          0,
	"TCP_FASTOPEN_ENABLED": 1,
}

func (x NodeAddress_Feature) String() string {
	return proto.EnumName(NodeAddress_Feature_name, int32(x))
}

func (NodeAddress_Feature) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{1, 0}
}

// Node represents a serialized NodeURL. A NodeURL should be able to be
// converted to a pb.Node and vice versa.
type Node struct {
	Id                   NodeID       `protobuf:"bytes,1,opt,name=id,proto3,customtype=NodeID" json:"id"`
	Address              *NodeAddress `protobuf:"bytes,2,opt,name=address,proto3" json:"address,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *Node) Reset()         { *m = Node{} }
func (m *Node) String() string { return proto.CompactTextString(m) }
func (*Node) ProtoMessage()    {}
func (*Node) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{0}
}
func (m *Node) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Node.Unmarshal(m, b)
}
func (m *Node) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Node.Marshal(b, m, deterministic)
}
func (m *Node) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Node.Merge(m, src)
}
func (m *Node) XXX_Size() int {
	return xxx_messageInfo_Node.Size(m)
}
func (m *Node) XXX_DiscardUnknown() {
	xxx_messageInfo_Node.DiscardUnknown(m)
}

var xxx_messageInfo_Node proto.InternalMessageInfo

func (m *Node) GetAddress() *NodeAddress {
	if m != nil {
		return m.Address
	}
	return nil
}

// NodeAddress contains the information needed to communicate with a node on
// the network.
type NodeAddress struct {
	Address   string     `protobuf:"bytes,2,opt,name=address,proto3" json:"address,omitempty"`
	NoiseInfo *NoiseInfo `protobuf:"bytes,3,opt,name=noise_info,json=noiseInfo,proto3" json:"noise_info,omitempty"`
	// the number of concurrent identical messages this node knows how to
	// debounce. restricted to a specific protocol (e.g. just TLS or just Noise).
	DebounceLimit        int32    `protobuf:"varint,4,opt,name=debounce_limit,json=debounceLimit,proto3" json:"debounce_limit,omitempty"`
	Features             uint64   `protobuf:"varint,5,opt,name=features,proto3" json:"features,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeAddress) Reset()         { *m = NodeAddress{} }
func (m *NodeAddress) String() string { return proto.CompactTextString(m) }
func (*NodeAddress) ProtoMessage()    {}
func (*NodeAddress) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{1}
}
func (m *NodeAddress) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeAddress.Unmarshal(m, b)
}
func (m *NodeAddress) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeAddress.Marshal(b, m, deterministic)
}
func (m *NodeAddress) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeAddress.Merge(m, src)
}
func (m *NodeAddress) XXX_Size() int {
	return xxx_messageInfo_NodeAddress.Size(m)
}
func (m *NodeAddress) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeAddress.DiscardUnknown(m)
}

var xxx_messageInfo_NodeAddress proto.InternalMessageInfo

func (m *NodeAddress) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func (m *NodeAddress) GetNoiseInfo() *NoiseInfo {
	if m != nil {
		return m.NoiseInfo
	}
	return nil
}

func (m *NodeAddress) GetDebounceLimit() int32 {
	if m != nil {
		return m.DebounceLimit
	}
	return 0
}

func (m *NodeAddress) GetFeatures() uint64 {
	if m != nil {
		return m.Features
	}
	return 0
}

// NodeOperator contains info about the storage node operator.
type NodeOperator struct {
	Email                string   `protobuf:"bytes,1,opt,name=email,proto3" json:"email,omitempty"`
	Wallet               string   `protobuf:"bytes,2,opt,name=wallet,proto3" json:"wallet,omitempty"`
	WalletFeatures       []string `protobuf:"bytes,3,rep,name=wallet_features,json=walletFeatures,proto3" json:"wallet_features,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeOperator) Reset()         { *m = NodeOperator{} }
func (m *NodeOperator) String() string { return proto.CompactTextString(m) }
func (*NodeOperator) ProtoMessage()    {}
func (*NodeOperator) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{2}
}
func (m *NodeOperator) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeOperator.Unmarshal(m, b)
}
func (m *NodeOperator) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeOperator.Marshal(b, m, deterministic)
}
func (m *NodeOperator) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeOperator.Merge(m, src)
}
func (m *NodeOperator) XXX_Size() int {
	return xxx_messageInfo_NodeOperator.Size(m)
}
func (m *NodeOperator) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeOperator.DiscardUnknown(m)
}

var xxx_messageInfo_NodeOperator proto.InternalMessageInfo

func (m *NodeOperator) GetEmail() string {
	if m != nil {
		return m.Email
	}
	return ""
}

func (m *NodeOperator) GetWallet() string {
	if m != nil {
		return m.Wallet
	}
	return ""
}

func (m *NodeOperator) GetWalletFeatures() []string {
	if m != nil {
		return m.WalletFeatures
	}
	return nil
}

// NodeCapacity contains all relevant data about a nodes ability to store data.
type NodeCapacity struct {
	FreeBandwidth        int64    `protobuf:"varint,1,opt,name=free_bandwidth,json=freeBandwidth,proto3" json:"free_bandwidth,omitempty"` // Deprecated: Do not use.
	FreeDisk             int64    `protobuf:"varint,2,opt,name=free_disk,json=freeDisk,proto3" json:"free_disk,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeCapacity) Reset()         { *m = NodeCapacity{} }
func (m *NodeCapacity) String() string { return proto.CompactTextString(m) }
func (*NodeCapacity) ProtoMessage()    {}
func (*NodeCapacity) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{3}
}
func (m *NodeCapacity) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeCapacity.Unmarshal(m, b)
}
func (m *NodeCapacity) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeCapacity.Marshal(b, m, deterministic)
}
func (m *NodeCapacity) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeCapacity.Merge(m, src)
}
func (m *NodeCapacity) XXX_Size() int {
	return xxx_messageInfo_NodeCapacity.Size(m)
}
func (m *NodeCapacity) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeCapacity.DiscardUnknown(m)
}

var xxx_messageInfo_NodeCapacity proto.InternalMessageInfo

// Deprecated: Do not use.
func (m *NodeCapacity) GetFreeBandwidth() int64 {
	if m != nil {
		return m.FreeBandwidth
	}
	return 0
}

func (m *NodeCapacity) GetFreeDisk() int64 {
	if m != nil {
		return m.FreeDisk
	}
	return 0
}

// Deprecated: use NodeOperator instead.
type NodeMetadata struct {
	Email                string   `protobuf:"bytes,1,opt,name=email,proto3" json:"email,omitempty"`
	Wallet               string   `protobuf:"bytes,2,opt,name=wallet,proto3" json:"wallet,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeMetadata) Reset()         { *m = NodeMetadata{} }
func (m *NodeMetadata) String() string { return proto.CompactTextString(m) }
func (*NodeMetadata) ProtoMessage()    {}
func (*NodeMetadata) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{4}
}
func (m *NodeMetadata) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeMetadata.Unmarshal(m, b)
}
func (m *NodeMetadata) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeMetadata.Marshal(b, m, deterministic)
}
func (m *NodeMetadata) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeMetadata.Merge(m, src)
}
func (m *NodeMetadata) XXX_Size() int {
	return xxx_messageInfo_NodeMetadata.Size(m)
}
func (m *NodeMetadata) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeMetadata.DiscardUnknown(m)
}

var xxx_messageInfo_NodeMetadata proto.InternalMessageInfo

func (m *NodeMetadata) GetEmail() string {
	if m != nil {
		return m.Email
	}
	return ""
}

func (m *NodeMetadata) GetWallet() string {
	if m != nil {
		return m.Wallet
	}
	return ""
}

// Deprecated: use NodeCapacity instead.
type NodeRestrictions struct {
	FreeBandwidth        int64    `protobuf:"varint,1,opt,name=free_bandwidth,json=freeBandwidth,proto3" json:"free_bandwidth,omitempty"`
	FreeDisk             int64    `protobuf:"varint,2,opt,name=free_disk,json=freeDisk,proto3" json:"free_disk,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *NodeRestrictions) Reset()         { *m = NodeRestrictions{} }
func (m *NodeRestrictions) String() string { return proto.CompactTextString(m) }
func (*NodeRestrictions) ProtoMessage()    {}
func (*NodeRestrictions) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{5}
}
func (m *NodeRestrictions) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeRestrictions.Unmarshal(m, b)
}
func (m *NodeRestrictions) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeRestrictions.Marshal(b, m, deterministic)
}
func (m *NodeRestrictions) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeRestrictions.Merge(m, src)
}
func (m *NodeRestrictions) XXX_Size() int {
	return xxx_messageInfo_NodeRestrictions.Size(m)
}
func (m *NodeRestrictions) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeRestrictions.DiscardUnknown(m)
}

var xxx_messageInfo_NodeRestrictions proto.InternalMessageInfo

func (m *NodeRestrictions) GetFreeBandwidth() int64 {
	if m != nil {
		return m.FreeBandwidth
	}
	return 0
}

func (m *NodeRestrictions) GetFreeDisk() int64 {
	if m != nil {
		return m.FreeDisk
	}
	return 0
}

// NodeVersion contains version information about a node.
type NodeVersion struct {
	Version              string    `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	CommitHash           string    `protobuf:"bytes,2,opt,name=commit_hash,json=commitHash,proto3" json:"commit_hash,omitempty"`
	Timestamp            time.Time `protobuf:"bytes,3,opt,name=timestamp,proto3,stdtime" json:"timestamp"`
	Release              bool      `protobuf:"varint,4,opt,name=release,proto3" json:"release,omitempty"`
	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	XXX_unrecognized     []byte    `json:"-"`
	XXX_sizecache        int32     `json:"-"`
}

func (m *NodeVersion) Reset()         { *m = NodeVersion{} }
func (m *NodeVersion) String() string { return proto.CompactTextString(m) }
func (*NodeVersion) ProtoMessage()    {}
func (*NodeVersion) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c843d59d2d938e7, []int{6}
}
func (m *NodeVersion) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_NodeVersion.Unmarshal(m, b)
}
func (m *NodeVersion) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_NodeVersion.Marshal(b, m, deterministic)
}
func (m *NodeVersion) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NodeVersion.Merge(m, src)
}
func (m *NodeVersion) XXX_Size() int {
	return xxx_messageInfo_NodeVersion.Size(m)
}
func (m *NodeVersion) XXX_DiscardUnknown() {
	xxx_messageInfo_NodeVersion.DiscardUnknown(m)
}

var xxx_messageInfo_NodeVersion proto.InternalMessageInfo

func (m *NodeVersion) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

func (m *NodeVersion) GetCommitHash() string {
	if m != nil {
		return m.CommitHash
	}
	return ""
}

func (m *NodeVersion) GetTimestamp() time.Time {
	if m != nil {
		return m.Timestamp
	}
	return time.Time{}
}

func (m *NodeVersion) GetRelease() bool {
	if m != nil {
		return m.Release
	}
	return false
}

func init() {
	proto.RegisterEnum("node.NodeType", NodeType_name, NodeType_value)
	proto.RegisterEnum("node.NodeTransport", NodeTransport_name, NodeTransport_value)
	proto.RegisterEnum("node.NodeAddress_Feature", NodeAddress_Feature_name, NodeAddress_Feature_value)
	proto.RegisterType((*Node)(nil), "node.Node")
	proto.RegisterType((*NodeAddress)(nil), "node.NodeAddress")
	proto.RegisterType((*NodeOperator)(nil), "node.NodeOperator")
	proto.RegisterType((*NodeCapacity)(nil), "node.NodeCapacity")
	proto.RegisterType((*NodeMetadata)(nil), "node.NodeMetadata")
	proto.RegisterType((*NodeRestrictions)(nil), "node.NodeRestrictions")
	proto.RegisterType((*NodeVersion)(nil), "node.NodeVersion")
}

func init() { proto.RegisterFile("node.proto", fileDescriptor_0c843d59d2d938e7) }

var fileDescriptor_0c843d59d2d938e7 = []byte{
	// 755 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x53, 0xcd, 0x6e, 0xeb, 0x44,
	0x14, 0xae, 0x1d, 0xb7, 0xb1, 0x4f, 0x7e, 0xea, 0x8e, 0x2a, 0x64, 0x05, 0x89, 0x44, 0x91, 0x10,
	0xe1, 0x22, 0x25, 0x12, 0xb0, 0x64, 0x63, 0xb7, 0x2e, 0xf8, 0x12, 0x9c, 0x30, 0x71, 0xbb, 0x60,
	0x63, 0x4d, 0xec, 0x49, 0x3b, 0x5c, 0xc7, 0x63, 0x79, 0xc6, 0x5c, 0x75, 0xcb, 0x13, 0xf0, 0x14,
	0x3c, 0x0b, 0xcf, 0xc0, 0xe2, 0x22, 0xf1, 0x24, 0x68, 0xfc, 0x13, 0xee, 0x95, 0xee, 0xa6, 0x1b,
	0x6b, 0xbe, 0xef, 0x9c, 0x99, 0xf3, 0x9d, 0xe3, 0xef, 0x00, 0xe4, 0x3c, 0xa5, 0xcb, 0xa2, 0xe4,
	0x92, 0x23, 0x43, 0x9d, 0x27, 0xf0, 0xc8, 0x1f, 0x79, 0xc3, 0x4c, 0xa6, 0x8f, 0x9c, 0x3f, 0x66,
	0x74, 0x55, 0xa3, 0x7d, 0x75, 0x58, 0x49, 0x76, 0xa4, 0x42, 0x92, 0x63, 0xd1, 0x26, 0x0c, 0x72,
	0xce, 0x44, 0x7b, 0x7f, 0xfe, 0xbb, 0x0e, 0x46, 0xc8, 0x53, 0x8a, 0x3e, 0x03, 0x9d, 0xa5, 0x8e,
	0x36, 0xd3, 0x16, 0x43, 0x6f, 0xfc, 0xd7, 0xbb, 0xe9, 0xd9, 0xdf, 0xef, 0xa6, 0x17, 0x2a, 0x12,
	0xdc, 0x62, 0x9d, 0xa5, 0xe8, 0x2b, 0xe8, 0x93, 0x34, 0x2d, 0xa9, 0x10, 0x8e, 0x3e, 0xd3, 0x16,
	0x83, 0xaf, 0xaf, 0x96, 0xb5, 0x0c, 0x95, 0xe2, 0x36, 0x01, 0xdc, 0x65, 0xbc, 0x36, 0xcc, 0x9e,
	0x7d, 0x89, 0x0d, 0xf9, 0x5c, 0x50, 0x3c, 0x2c, 0xa9, 0x90, 0x25, 0x4b, 0x24, 0xe3, 0xb9, 0xc0,
	0x50, 0xd2, 0xa2, 0x92, 0x44, 0x01, 0x6c, 0x1e, 0xa9, 0x24, 0x29, 0x91, 0x04, 0x0f, 0x33, 0x22,
	0x69, 0x9e, 0x3c, 0xc7, 0x19, 0x13, 0x12, 0x8f, 0x48, 0x95, 0x32, 0x19, 0x8b, 0x2a, 0x49, 0xd4,
	0xab, 0xe7, 0x4c, 0xc4, 0x55, 0x81, 0xc7, 0x55, 0x91, 0x12, 0x49, 0xe3, 0x36, 0x15, 0x5f, 0xb7,
	0xf8, 0xc3, 0xe4, 0x51, 0xcb, 0x56, 0x85, 0x6a, 0x1b, 0xf7, 0x7f, 0xa3, 0xa5, 0x50, 0xb5, 0xfa,
	0x19, 0x11, 0x32, 0x66, 0x05, 0x46, 0x29, 0x2d, 0x4a, 0x9a, 0x10, 0x49, 0xd3, 0xb8, 0xe5, 0xe6,
	0xff, 0x6a, 0x30, 0x78, 0xaf, 0x0f, 0xe4, 0x7c, 0xd8, 0xab, 0x75, 0x6a, 0x0c, 0xad, 0xd4, 0xf0,
	0x99, 0xa0, 0x31, 0xcb, 0x0f, 0xdc, 0xe9, 0xd5, 0x83, 0xb0, 0x97, 0xcd, 0x40, 0x43, 0xf5, 0x0d,
	0xf2, 0x03, 0xc7, 0x56, 0xde, 0x1d, 0xd1, 0xe7, 0x30, 0x4e, 0xe9, 0x9e, 0x57, 0x79, 0x42, 0xe3,
	0x8c, 0x1d, 0x99, 0x74, 0x8c, 0x99, 0xb6, 0x38, 0xc7, 0xa3, 0x8e, 0x5d, 0x2b, 0x12, 0x4d, 0xc0,
	0x3c, 0x50, 0x22, 0xab, 0x92, 0x0a, 0xe7, 0x7c, 0xa6, 0x2d, 0x0c, 0x7c, 0xc2, 0xf3, 0x6f, 0xa1,
	0x7f, 0xd7, 0x9c, 0xd1, 0x25, 0x0c, 0xc2, 0x4d, 0x7c, 0xe7, 0xbb, 0xd1, 0x3d, 0xf6, 0x77, 0xf6,
	0x19, 0x72, 0xe0, 0x3a, 0xba, 0xd9, 0xc6, 0x77, 0xee, 0x2e, 0xda, 0x6c, 0xfd, 0x30, 0xf6, 0x43,
	0xd7, 0x5b, 0xfb, 0xb7, 0xb6, 0xf6, 0xda, 0x30, 0x35, 0x5b, 0xc7, 0x96, 0x2c, 0x49, 0x2e, 0x0a,
	0x5e, 0xca, 0x39, 0x85, 0xa1, 0xea, 0x71, 0x53, 0xd0, 0x92, 0x48, 0x5e, 0xa2, 0x6b, 0x38, 0xa7,
	0x47, 0xc2, 0xb2, 0xfa, 0x9f, 0x5b, 0xb8, 0x01, 0xe8, 0x13, 0xb8, 0x78, 0x4b, 0xb2, 0x8c, 0xca,
	0xb6, 0xf3, 0x16, 0xa1, 0x2f, 0xe0, 0xb2, 0x39, 0xc5, 0x27, 0x9d, 0xbd, 0x59, 0x6f, 0x61, 0xe1,
	0x71, 0x43, 0xdf, 0x75, 0x6a, 0x1f, 0x9a, 0x32, 0x37, 0xa4, 0x20, 0x09, 0x93, 0xcf, 0xe8, 0x4b,
	0x18, 0x1f, 0x4a, 0x4a, 0xe3, 0x3d, 0xc9, 0xd3, 0xb7, 0x2c, 0x95, 0x4f, 0x75, 0xbd, 0x9e, 0xa7,
	0x3b, 0x1a, 0x1e, 0xa9, 0x88, 0xd7, 0x05, 0xd0, 0xa7, 0x60, 0xd5, 0xa9, 0x29, 0x13, 0x6f, 0xea,
	0xf2, 0x3d, 0x6c, 0x2a, 0xe2, 0x96, 0x89, 0x37, 0xf3, 0xef, 0x9a, 0x77, 0x7f, 0x6a, 0x2d, 0xf3,
	0x32, 0xf9, 0xf3, 0x07, 0xb0, 0xd5, 0x6d, 0xfc, 0x9e, 0x15, 0xd5, 0xaf, 0xf9, 0x98, 0xb2, 0x17,
	0xa9, 0xfa, 0xb3, 0x75, 0xce, 0x43, 0x63, 0x33, 0xe5, 0x9c, 0xd6, 0x71, 0xad, 0xae, 0x0e, 0xa2,
	0x29, 0x0c, 0x12, 0x7e, 0x3c, 0x32, 0x19, 0x3f, 0x11, 0xf1, 0xd4, 0xca, 0x83, 0x86, 0xfa, 0x81,
	0x88, 0x27, 0xe4, 0x81, 0x75, 0xda, 0xd4, 0xd6, 0x59, 0x93, 0x65, 0xb3, 0xcb, 0xcb, 0x6e, 0x97,
	0x97, 0x51, 0x97, 0xe1, 0x99, 0x6a, 0x47, 0xff, 0xf8, 0x67, 0xaa, 0xe1, 0xff, 0xaf, 0xa9, 0xf2,
	0x25, 0xcd, 0x28, 0x11, 0xb4, 0xb6, 0x99, 0x89, 0x3b, 0xf8, 0x0a, 0x83, 0xa9, 0x74, 0x46, 0xcf,
	0x05, 0x45, 0x03, 0xe8, 0x07, 0xe1, 0x83, 0xbb, 0x0e, 0x6e, 0xed, 0x33, 0x34, 0x02, 0x6b, 0xe7,
	0x46, 0xfe, 0x7a, 0x1d, 0x44, 0xbe, 0xad, 0xa9, 0xd8, 0x2e, 0xda, 0x60, 0xf7, 0x7b, 0xdf, 0xd6,
	0x11, 0xc0, 0xc5, 0xfd, 0x76, 0x1d, 0x84, 0x3f, 0xda, 0x3d, 0x74, 0x05, 0x96, 0xb7, 0xd9, 0x44,
	0xbb, 0x08, 0xbb, 0x5b, 0xdb, 0x98, 0xe8, 0xa6, 0xf6, 0xca, 0x85, 0x51, 0xfd, 0x66, 0x67, 0x31,
	0x65, 0x4f, 0xe5, 0xc6, 0x68, 0xbd, 0x8b, 0xf1, 0xf6, 0xc6, 0x3e, 0x43, 0x43, 0x30, 0x7f, 0xbe,
	0x0f, 0x6e, 0x6a, 0xa4, 0xa1, 0x2b, 0x18, 0xa9, 0x70, 0xb8, 0x09, 0x76, 0x7e, 0x4d, 0xe9, 0xde,
	0xf5, 0x2f, 0x48, 0x48, 0x5e, 0xfe, 0xba, 0x64, 0x7c, 0xa5, 0x66, 0xc1, 0xf3, 0x55, 0xb1, 0xdf,
	0x5f, 0xd4, 0xfd, 0x7e, 0xf3, 0x5f, 0x00, 0x00, 0x00, 0xff, 0xff, 0x86, 0x1e, 0x5f, 0xb7, 0xe9,
	0x04, 0x00, 0x00,
}