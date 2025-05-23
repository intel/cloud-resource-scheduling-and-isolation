/*
Copyright (C) 2025 Intel Corporation
SPDX-License-Identifier: Apache-2.0
*/

//
// nodeagent.proto
//
// communicate with aggregator
//

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.15.8
// source: pkg/api/nodeagent/nodeagent.proto

package nodeagent

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	aggregator "sigs.k8s.io/IOIsolation/pkg/api/aggregator"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type UpdateReservedPodsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NodeName     string              `protobuf:"bytes,1,opt,name=node_name,json=nodeName,proto3" json:"node_name,omitempty"`
	ReservedPods *aggregator.PodList `protobuf:"bytes,2,opt,name=reserved_pods,json=reservedPods,proto3" json:"reserved_pods,omitempty"`
}

func (x *UpdateReservedPodsRequest) Reset() {
	*x = UpdateReservedPodsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_api_nodeagent_nodeagent_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateReservedPodsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateReservedPodsRequest) ProtoMessage() {}

func (x *UpdateReservedPodsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_api_nodeagent_nodeagent_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateReservedPodsRequest.ProtoReflect.Descriptor instead.
func (*UpdateReservedPodsRequest) Descriptor() ([]byte, []int) {
	return file_pkg_api_nodeagent_nodeagent_proto_rawDescGZIP(), []int{0}
}

func (x *UpdateReservedPodsRequest) GetNodeName() string {
	if x != nil {
		return x.NodeName
	}
	return ""
}

func (x *UpdateReservedPodsRequest) GetReservedPods() *aggregator.PodList {
	if x != nil {
		return x.ReservedPods
	}
	return nil
}

type GetNodeIOStatusRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type aggregator.IOType `protobuf:"varint,1,opt,name=type,proto3,enum=aggregator.IOType" json:"type,omitempty"`
}

func (x *GetNodeIOStatusRequest) Reset() {
	*x = GetNodeIOStatusRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_api_nodeagent_nodeagent_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetNodeIOStatusRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetNodeIOStatusRequest) ProtoMessage() {}

func (x *GetNodeIOStatusRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_api_nodeagent_nodeagent_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetNodeIOStatusRequest.ProtoReflect.Descriptor instead.
func (*GetNodeIOStatusRequest) Descriptor() ([]byte, []int) {
	return file_pkg_api_nodeagent_nodeagent_proto_rawDescGZIP(), []int{1}
}

func (x *GetNodeIOStatusRequest) GetType() aggregator.IOType {
	if x != nil {
		return x.Type
	}
	return aggregator.IOType(0)
}

type GetNodeIOStatusResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NodeIoBw *aggregator.IOBandwidth `protobuf:"bytes,1,opt,name=node_io_bw,json=nodeIoBw,proto3" json:"node_io_bw,omitempty"`
}

func (x *GetNodeIOStatusResponse) Reset() {
	*x = GetNodeIOStatusResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_api_nodeagent_nodeagent_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetNodeIOStatusResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetNodeIOStatusResponse) ProtoMessage() {}

func (x *GetNodeIOStatusResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_api_nodeagent_nodeagent_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetNodeIOStatusResponse.ProtoReflect.Descriptor instead.
func (*GetNodeIOStatusResponse) Descriptor() ([]byte, []int) {
	return file_pkg_api_nodeagent_nodeagent_proto_rawDescGZIP(), []int{2}
}

func (x *GetNodeIOStatusResponse) GetNodeIoBw() *aggregator.IOBandwidth {
	if x != nil {
		return x.NodeIoBw
	}
	return nil
}

var File_pkg_api_nodeagent_nodeagent_proto protoreflect.FileDescriptor

var file_pkg_api_nodeagent_nodeagent_proto_rawDesc = []byte{
	0x0a, 0x21, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x61, 0x67,
	0x65, 0x6e, 0x74, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x09, 0x6e, 0x6f, 0x64, 0x65, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x1a, 0x23,
	0x70, 0x6b, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74,
	0x6f, 0x72, 0x2f, 0x61, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x72, 0x0a, 0x19, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73,
	0x65, 0x72, 0x76, 0x65, 0x64, 0x50, 0x6f, 0x64, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x1b, 0x0a, 0x09, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x6e, 0x6f, 0x64, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x38, 0x0a,
	0x0d, 0x72, 0x65, 0x73, 0x65, 0x72, 0x76, 0x65, 0x64, 0x5f, 0x70, 0x6f, 0x64, 0x73, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x13, 0x2e, 0x61, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x6f,
	0x72, 0x2e, 0x50, 0x6f, 0x64, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x0c, 0x72, 0x65, 0x73, 0x65, 0x72,
	0x76, 0x65, 0x64, 0x50, 0x6f, 0x64, 0x73, 0x22, 0x40, 0x0a, 0x16, 0x47, 0x65, 0x74, 0x4e, 0x6f,
	0x64, 0x65, 0x49, 0x4f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x26, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x12, 0x2e, 0x61, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x6f, 0x72, 0x2e, 0x49, 0x4f, 0x54,
	0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x50, 0x0a, 0x17, 0x47, 0x65, 0x74,
	0x4e, 0x6f, 0x64, 0x65, 0x49, 0x4f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x35, 0x0a, 0x0a, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x6f, 0x5f,
	0x62, 0x77, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x61, 0x67, 0x67, 0x72, 0x65,
	0x67, 0x61, 0x74, 0x6f, 0x72, 0x2e, 0x49, 0x4f, 0x42, 0x61, 0x6e, 0x64, 0x77, 0x69, 0x64, 0x74,
	0x68, 0x52, 0x08, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x6f, 0x42, 0x77, 0x32, 0xb4, 0x01, 0x0a, 0x09,
	0x4e, 0x6f, 0x64, 0x65, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x12, 0x4d, 0x0a, 0x12, 0x55, 0x70, 0x64,
	0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x65, 0x72, 0x76, 0x65, 0x64, 0x50, 0x6f, 0x64, 0x73, 0x12,
	0x24, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x55, 0x70, 0x64, 0x61,
	0x74, 0x65, 0x52, 0x65, 0x73, 0x65, 0x72, 0x76, 0x65, 0x64, 0x50, 0x6f, 0x64, 0x73, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x11, 0x2e, 0x61, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74,
	0x6f, 0x72, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x12, 0x58, 0x0a, 0x0f, 0x47, 0x65, 0x74, 0x4e,
	0x6f, 0x64, 0x65, 0x49, 0x4f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x21, 0x2e, 0x6e, 0x6f,
	0x64, 0x65, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x47, 0x65, 0x74, 0x4e, 0x6f, 0x64, 0x65, 0x49,
	0x4f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x22,
	0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x47, 0x65, 0x74, 0x4e, 0x6f,
	0x64, 0x65, 0x49, 0x4f, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x42, 0x35, 0x5a, 0x33, 0x73, 0x69, 0x67, 0x73, 0x2e, 0x6b, 0x38, 0x73, 0x2e, 0x69,
	0x6f, 0x2f, 0x49, 0x4f, 0x49, 0x73, 0x6f, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x70, 0x6b,
	0x67, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x3b,
	0x6e, 0x6f, 0x64, 0x65, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_pkg_api_nodeagent_nodeagent_proto_rawDescOnce sync.Once
	file_pkg_api_nodeagent_nodeagent_proto_rawDescData = file_pkg_api_nodeagent_nodeagent_proto_rawDesc
)

func file_pkg_api_nodeagent_nodeagent_proto_rawDescGZIP() []byte {
	file_pkg_api_nodeagent_nodeagent_proto_rawDescOnce.Do(func() {
		file_pkg_api_nodeagent_nodeagent_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_api_nodeagent_nodeagent_proto_rawDescData)
	})
	return file_pkg_api_nodeagent_nodeagent_proto_rawDescData
}

var file_pkg_api_nodeagent_nodeagent_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_pkg_api_nodeagent_nodeagent_proto_goTypes = []interface{}{
	(*UpdateReservedPodsRequest)(nil), // 0: nodeagent.UpdateReservedPodsRequest
	(*GetNodeIOStatusRequest)(nil),    // 1: nodeagent.GetNodeIOStatusRequest
	(*GetNodeIOStatusResponse)(nil),   // 2: nodeagent.GetNodeIOStatusResponse
	(*aggregator.PodList)(nil),        // 3: aggregator.PodList
	(aggregator.IOType)(0),            // 4: aggregator.IOType
	(*aggregator.IOBandwidth)(nil),    // 5: aggregator.IOBandwidth
	(*aggregator.Empty)(nil),          // 6: aggregator.Empty
}
var file_pkg_api_nodeagent_nodeagent_proto_depIdxs = []int32{
	3, // 0: nodeagent.UpdateReservedPodsRequest.reserved_pods:type_name -> aggregator.PodList
	4, // 1: nodeagent.GetNodeIOStatusRequest.type:type_name -> aggregator.IOType
	5, // 2: nodeagent.GetNodeIOStatusResponse.node_io_bw:type_name -> aggregator.IOBandwidth
	0, // 3: nodeagent.NodeAgent.UpdateReservedPods:input_type -> nodeagent.UpdateReservedPodsRequest
	1, // 4: nodeagent.NodeAgent.GetNodeIOStatus:input_type -> nodeagent.GetNodeIOStatusRequest
	6, // 5: nodeagent.NodeAgent.UpdateReservedPods:output_type -> aggregator.Empty
	2, // 6: nodeagent.NodeAgent.GetNodeIOStatus:output_type -> nodeagent.GetNodeIOStatusResponse
	5, // [5:7] is the sub-list for method output_type
	3, // [3:5] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_pkg_api_nodeagent_nodeagent_proto_init() }
func file_pkg_api_nodeagent_nodeagent_proto_init() {
	if File_pkg_api_nodeagent_nodeagent_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_api_nodeagent_nodeagent_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateReservedPodsRequest); i {
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
		file_pkg_api_nodeagent_nodeagent_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetNodeIOStatusRequest); i {
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
		file_pkg_api_nodeagent_nodeagent_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetNodeIOStatusResponse); i {
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
			RawDescriptor: file_pkg_api_nodeagent_nodeagent_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pkg_api_nodeagent_nodeagent_proto_goTypes,
		DependencyIndexes: file_pkg_api_nodeagent_nodeagent_proto_depIdxs,
		MessageInfos:      file_pkg_api_nodeagent_nodeagent_proto_msgTypes,
	}.Build()
	File_pkg_api_nodeagent_nodeagent_proto = out.File
	file_pkg_api_nodeagent_nodeagent_proto_rawDesc = nil
	file_pkg_api_nodeagent_nodeagent_proto_goTypes = nil
	file_pkg_api_nodeagent_nodeagent_proto_depIdxs = nil
}
