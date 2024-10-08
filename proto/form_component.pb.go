﻿// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.20.3
// source: form_component.proto

package usercenter

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

type FormComponentInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id              string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	Name            string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name"`
	Group           string                 `protobuf:"bytes,3,opt,name=group,proto3" json:"group"`
	Index           int32                  `protobuf:"varint,4,opt,name=index,proto3" json:"index"`
	Description     string                 `protobuf:"bytes,5,opt,name=description,proto3" json:"description"`
	Extends         string                 `protobuf:"bytes,6,opt,name=extends,proto3" json:"extends"`
	Selector        string                 `protobuf:"bytes,7,opt,name=selector,proto3" json:"selector"`
	DesignerProps   string                 `protobuf:"bytes,8,opt,name=designerProps,proto3" json:"designerProps"`
	DesignerLocales string                 `protobuf:"bytes,9,opt,name=designerLocales,proto3" json:"designerLocales"`
	Resource        *FormComponentResource `protobuf:"bytes,10,opt,name=resource,proto3" json:"resource"`
	Title           string                 `protobuf:"bytes,11,opt,name=title,proto3" json:"title"`
	Byo             bool                   `protobuf:"varint,12,opt,name=byo,proto3" json:"byo"`
}

func (x *FormComponentInfo) Reset() {
	*x = FormComponentInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_form_component_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FormComponentInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FormComponentInfo) ProtoMessage() {}

func (x *FormComponentInfo) ProtoReflect() protoreflect.Message {
	mi := &file_form_component_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FormComponentInfo.ProtoReflect.Descriptor instead.
func (*FormComponentInfo) Descriptor() ([]byte, []int) {
	return file_form_component_proto_rawDescGZIP(), []int{0}
}

func (x *FormComponentInfo) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *FormComponentInfo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *FormComponentInfo) GetGroup() string {
	if x != nil {
		return x.Group
	}
	return ""
}

func (x *FormComponentInfo) GetIndex() int32 {
	if x != nil {
		return x.Index
	}
	return 0
}

func (x *FormComponentInfo) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *FormComponentInfo) GetExtends() string {
	if x != nil {
		return x.Extends
	}
	return ""
}

func (x *FormComponentInfo) GetSelector() string {
	if x != nil {
		return x.Selector
	}
	return ""
}

func (x *FormComponentInfo) GetDesignerProps() string {
	if x != nil {
		return x.DesignerProps
	}
	return ""
}

func (x *FormComponentInfo) GetDesignerLocales() string {
	if x != nil {
		return x.DesignerLocales
	}
	return ""
}

func (x *FormComponentInfo) GetResource() *FormComponentResource {
	if x != nil {
		return x.Resource
	}
	return nil
}

func (x *FormComponentInfo) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *FormComponentInfo) GetByo() bool {
	if x != nil {
		return x.Byo
	}
	return false
}

type FormComponentResource struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id              string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	FormComponentID string `protobuf:"bytes,2,opt,name=formComponentID,proto3" json:"formComponentID"`
	Icon            string `protobuf:"bytes,3,opt,name=icon,proto3" json:"icon"`
	Thumb           string `protobuf:"bytes,4,opt,name=thumb,proto3" json:"thumb"`
	Title           string `protobuf:"bytes,5,opt,name=title,proto3" json:"title"`
	Description     string `protobuf:"bytes,6,opt,name=description,proto3" json:"description"`
	Span            int32  `protobuf:"varint,7,opt,name=span,proto3" json:"span"`
	Elements        string `protobuf:"bytes,8,opt,name=elements,proto3" json:"elements"`
}

func (x *FormComponentResource) Reset() {
	*x = FormComponentResource{}
	if protoimpl.UnsafeEnabled {
		mi := &file_form_component_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FormComponentResource) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FormComponentResource) ProtoMessage() {}

func (x *FormComponentResource) ProtoReflect() protoreflect.Message {
	mi := &file_form_component_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FormComponentResource.ProtoReflect.Descriptor instead.
func (*FormComponentResource) Descriptor() ([]byte, []int) {
	return file_form_component_proto_rawDescGZIP(), []int{1}
}

func (x *FormComponentResource) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *FormComponentResource) GetFormComponentID() string {
	if x != nil {
		return x.FormComponentID
	}
	return ""
}

func (x *FormComponentResource) GetIcon() string {
	if x != nil {
		return x.Icon
	}
	return ""
}

func (x *FormComponentResource) GetThumb() string {
	if x != nil {
		return x.Thumb
	}
	return ""
}

func (x *FormComponentResource) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *FormComponentResource) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *FormComponentResource) GetSpan() int32 {
	if x != nil {
		return x.Span
	}
	return 0
}

func (x *FormComponentResource) GetElements() string {
	if x != nil {
		return x.Elements
	}
	return ""
}

type QueryFormComponentRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// @inject_tag: uri:"pageIndex" form:"pageIndex"
	PageIndex int64 `protobuf:"varint,1,opt,name=pageIndex,proto3" json:"pageIndex" uri:"pageIndex" form:"pageIndex"`
	// @inject_tag: uri:"pageSize" form:"pageSize"
	PageSize int64 `protobuf:"varint,2,opt,name=pageSize,proto3" json:"pageSize" uri:"pageSize" form:"pageSize"`
	// @inject_tag: uri:"orderField" form:"orderField"
	OrderField string `protobuf:"bytes,3,opt,name=orderField,proto3" json:"orderField" uri:"orderField" form:"orderField"`
	// @inject_tag: uri:"desc" form:"desc"
	Desc bool `protobuf:"varint,4,opt,name=desc,proto3" json:"desc" uri:"desc" form:"desc"`
	// @inject_tag: uri:"name" form:"name"
	Name string `protobuf:"bytes,5,opt,name=name,proto3" json:"name" uri:"name" form:"name"`
	// @inject_tag: uri:"group" form:"group"
	Group string `protobuf:"bytes,6,opt,name=group,proto3" json:"group" uri:"group" form:"group"`
	// @inject_tag: uri:"sortConfig" form:"sortConfig"
	SortConfig string `protobuf:"bytes,7,opt,name=sortConfig,proto3" json:"sortConfig" uri:"sortConfig" form:"sortConfig"`
}

func (x *QueryFormComponentRequest) Reset() {
	*x = QueryFormComponentRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_form_component_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueryFormComponentRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryFormComponentRequest) ProtoMessage() {}

func (x *QueryFormComponentRequest) ProtoReflect() protoreflect.Message {
	mi := &file_form_component_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryFormComponentRequest.ProtoReflect.Descriptor instead.
func (*QueryFormComponentRequest) Descriptor() ([]byte, []int) {
	return file_form_component_proto_rawDescGZIP(), []int{2}
}

func (x *QueryFormComponentRequest) GetPageIndex() int64 {
	if x != nil {
		return x.PageIndex
	}
	return 0
}

func (x *QueryFormComponentRequest) GetPageSize() int64 {
	if x != nil {
		return x.PageSize
	}
	return 0
}

func (x *QueryFormComponentRequest) GetOrderField() string {
	if x != nil {
		return x.OrderField
	}
	return ""
}

func (x *QueryFormComponentRequest) GetDesc() bool {
	if x != nil {
		return x.Desc
	}
	return false
}

func (x *QueryFormComponentRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *QueryFormComponentRequest) GetGroup() string {
	if x != nil {
		return x.Group
	}
	return ""
}

func (x *QueryFormComponentRequest) GetSortConfig() string {
	if x != nil {
		return x.SortConfig
	}
	return ""
}

type QueryFormComponentResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code                 `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string               `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    []*FormComponentInfo `protobuf:"bytes,3,rep,name=data,proto3" json:"data"`
	Pages   int64                `protobuf:"varint,4,opt,name=pages,proto3" json:"pages"`
	Records int64                `protobuf:"varint,5,opt,name=records,proto3" json:"records"`
	Total   int64                `protobuf:"varint,6,opt,name=total,proto3" json:"total"`
}

func (x *QueryFormComponentResponse) Reset() {
	*x = QueryFormComponentResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_form_component_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueryFormComponentResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryFormComponentResponse) ProtoMessage() {}

func (x *QueryFormComponentResponse) ProtoReflect() protoreflect.Message {
	mi := &file_form_component_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryFormComponentResponse.ProtoReflect.Descriptor instead.
func (*QueryFormComponentResponse) Descriptor() ([]byte, []int) {
	return file_form_component_proto_rawDescGZIP(), []int{3}
}

func (x *QueryFormComponentResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *QueryFormComponentResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *QueryFormComponentResponse) GetData() []*FormComponentInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *QueryFormComponentResponse) GetPages() int64 {
	if x != nil {
		return x.Pages
	}
	return 0
}

func (x *QueryFormComponentResponse) GetRecords() int64 {
	if x != nil {
		return x.Records
	}
	return 0
}

func (x *QueryFormComponentResponse) GetTotal() int64 {
	if x != nil {
		return x.Total
	}
	return 0
}

type GetFormComponentDetailResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code               `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string             `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    *FormComponentInfo `protobuf:"bytes,3,opt,name=data,proto3" json:"data"`
}

func (x *GetFormComponentDetailResponse) Reset() {
	*x = GetFormComponentDetailResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_form_component_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetFormComponentDetailResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetFormComponentDetailResponse) ProtoMessage() {}

func (x *GetFormComponentDetailResponse) ProtoReflect() protoreflect.Message {
	mi := &file_form_component_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetFormComponentDetailResponse.ProtoReflect.Descriptor instead.
func (*GetFormComponentDetailResponse) Descriptor() ([]byte, []int) {
	return file_form_component_proto_rawDescGZIP(), []int{4}
}

func (x *GetFormComponentDetailResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *GetFormComponentDetailResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *GetFormComponentDetailResponse) GetData() *FormComponentInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_form_component_proto protoreflect.FileDescriptor

var file_form_component_proto_rawDesc = []byte{
	0x0a, 0x14, 0x66, 0x6f, 0x72, 0x6d, 0x5f, 0x63, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0a, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x65, 0x72, 0x1a, 0x0c, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0xf2, 0x02, 0x0a, 0x11, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65,
	0x6e, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x67, 0x72,
	0x6f, 0x75, 0x70, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x67, 0x72, 0x6f, 0x75, 0x70,
	0x12, 0x14, 0x0a, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x65, 0x78, 0x74, 0x65,
	0x6e, 0x64, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x65, 0x78, 0x74, 0x65, 0x6e,
	0x64, 0x73, 0x12, 0x1a, 0x0a, 0x08, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x18, 0x07,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x73, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x12, 0x24,
	0x0a, 0x0d, 0x64, 0x65, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x50, 0x72, 0x6f, 0x70, 0x73, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x64, 0x65, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x50,
	0x72, 0x6f, 0x70, 0x73, 0x12, 0x28, 0x0a, 0x0f, 0x64, 0x65, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72,
	0x4c, 0x6f, 0x63, 0x61, 0x6c, 0x65, 0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x64,
	0x65, 0x73, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x4c, 0x6f, 0x63, 0x61, 0x6c, 0x65, 0x73, 0x12, 0x3d,
	0x0a, 0x08, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x21, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x46, 0x6f,
	0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x52, 0x08, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x12, 0x14, 0x0a,
	0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x69,
	0x74, 0x6c, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x62, 0x79, 0x6f, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x03, 0x62, 0x79, 0x6f, 0x22, 0xe3, 0x01, 0x0a, 0x15, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f,
	0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x12,
	0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12,
	0x28, 0x0a, 0x0f, 0x66, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74,
	0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x66, 0x6f, 0x72, 0x6d, 0x43, 0x6f,
	0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x12, 0x0a, 0x04, 0x69, 0x63, 0x6f,
	0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x69, 0x63, 0x6f, 0x6e, 0x12, 0x14, 0x0a,
	0x05, 0x74, 0x68, 0x75, 0x6d, 0x62, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x68,
	0x75, 0x6d, 0x62, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b,
	0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x73,
	0x70, 0x61, 0x6e, 0x18, 0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x73, 0x70, 0x61, 0x6e, 0x12,
	0x1a, 0x0a, 0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x08, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x65, 0x6c, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x73, 0x22, 0xd3, 0x01, 0x0a, 0x19,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65,
	0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x61, 0x67,
	0x65, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x70, 0x61,
	0x67, 0x65, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x61, 0x67, 0x65, 0x53,
	0x69, 0x7a, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x70, 0x61, 0x67, 0x65, 0x53,
	0x69, 0x7a, 0x65, 0x12, 0x1e, 0x0a, 0x0a, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x46, 0x69, 0x65, 0x6c,
	0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x46, 0x69,
	0x65, 0x6c, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x65, 0x73, 0x63, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x04, 0x64, 0x65, 0x73, 0x63, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x67,
	0x72, 0x6f, 0x75, 0x70, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x67, 0x72, 0x6f, 0x75,
	0x70, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x6f, 0x72, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x73, 0x6f, 0x72, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x22, 0xd5, 0x01, 0x0a, 0x1a, 0x51, 0x75, 0x65, 0x72, 0x79, 0x46, 0x6f, 0x72, 0x6d, 0x43,
	0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x24, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x10,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x64, 0x65,
	0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x12, 0x31, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1d,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x46, 0x6f, 0x72, 0x6d,
	0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x70, 0x61, 0x67, 0x65, 0x73, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x05, 0x70, 0x61, 0x67, 0x65, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x72, 0x65, 0x63,
	0x6f, 0x72, 0x64, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x72, 0x65, 0x63, 0x6f,
	0x72, 0x64, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x22, 0x93, 0x01, 0x0a, 0x1e, 0x47, 0x65,
	0x74, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x44, 0x65,
	0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x24, 0x0a, 0x04,
	0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x10, 0x2e, 0x75, 0x73, 0x65,
	0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x63, 0x6f,
	0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x31, 0x0a, 0x04,
	0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x75, 0x73, 0x65,
	0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70,
	0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x32,
	0x8d, 0x03, 0x0a, 0x0d, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e,
	0x74, 0x12, 0x42, 0x0a, 0x03, 0x41, 0x64, 0x64, 0x12, 0x1d, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63,
	0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e,
	0x65, 0x6e, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x1a, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x45, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12,
	0x1d, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x46, 0x6f, 0x72,
	0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x1a,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d,
	0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3e, 0x0a, 0x06,
	0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x12, 0x16, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e,
	0x74, 0x65, 0x72, 0x2e, 0x44, 0x65, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d,
	0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x58, 0x0a, 0x05,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x12, 0x25, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x65, 0x72, 0x2e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70,
	0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x26, 0x2e, 0x75,
	0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x46,
	0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x57, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x44, 0x65, 0x74,
	0x61, 0x69, 0x6c, 0x12, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72,
	0x2e, 0x47, 0x65, 0x74, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x2a, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x47,
	0x65, 0x74, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x44,
	0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42,
	0x4d, 0x0a, 0x13, 0x63, 0x6e, 0x2e, 0x61, 0x74, 0x61, 0x6c, 0x69, 0x2e, 0x75, 0x73, 0x65, 0x72,
	0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x42, 0x12, 0x46, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70,
	0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x0d, 0x2e, 0x2f,
	0x3b, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0xa2, 0x02, 0x10, 0x46, 0x4f,
	0x52, 0x4d, 0x43, 0x4f, 0x4d, 0x50, 0x4f, 0x4e, 0x45, 0x4e, 0x54, 0x53, 0x52, 0x56, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_form_component_proto_rawDescOnce sync.Once
	file_form_component_proto_rawDescData = file_form_component_proto_rawDesc
)

func file_form_component_proto_rawDescGZIP() []byte {
	file_form_component_proto_rawDescOnce.Do(func() {
		file_form_component_proto_rawDescData = protoimpl.X.CompressGZIP(file_form_component_proto_rawDescData)
	})
	return file_form_component_proto_rawDescData
}

var file_form_component_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_form_component_proto_goTypes = []interface{}{
	(*FormComponentInfo)(nil),              // 0: usercenter.FormComponentInfo
	(*FormComponentResource)(nil),          // 1: usercenter.FormComponentResource
	(*QueryFormComponentRequest)(nil),      // 2: usercenter.QueryFormComponentRequest
	(*QueryFormComponentResponse)(nil),     // 3: usercenter.QueryFormComponentResponse
	(*GetFormComponentDetailResponse)(nil), // 4: usercenter.GetFormComponentDetailResponse
	(Code)(0),                              // 5: usercenter.Code
	(*DelRequest)(nil),                     // 6: usercenter.DelRequest
	(*GetDetailRequest)(nil),               // 7: usercenter.GetDetailRequest
	(*CommonResponse)(nil),                 // 8: usercenter.CommonResponse
}
var file_form_component_proto_depIdxs = []int32{
	1,  // 0: usercenter.FormComponentInfo.resource:type_name -> usercenter.FormComponentResource
	5,  // 1: usercenter.QueryFormComponentResponse.code:type_name -> usercenter.Code
	0,  // 2: usercenter.QueryFormComponentResponse.data:type_name -> usercenter.FormComponentInfo
	5,  // 3: usercenter.GetFormComponentDetailResponse.code:type_name -> usercenter.Code
	0,  // 4: usercenter.GetFormComponentDetailResponse.data:type_name -> usercenter.FormComponentInfo
	0,  // 5: usercenter.FormComponent.Add:input_type -> usercenter.FormComponentInfo
	0,  // 6: usercenter.FormComponent.Update:input_type -> usercenter.FormComponentInfo
	6,  // 7: usercenter.FormComponent.Delete:input_type -> usercenter.DelRequest
	2,  // 8: usercenter.FormComponent.Query:input_type -> usercenter.QueryFormComponentRequest
	7,  // 9: usercenter.FormComponent.GetDetail:input_type -> usercenter.GetDetailRequest
	8,  // 10: usercenter.FormComponent.Add:output_type -> usercenter.CommonResponse
	8,  // 11: usercenter.FormComponent.Update:output_type -> usercenter.CommonResponse
	8,  // 12: usercenter.FormComponent.Delete:output_type -> usercenter.CommonResponse
	3,  // 13: usercenter.FormComponent.Query:output_type -> usercenter.QueryFormComponentResponse
	4,  // 14: usercenter.FormComponent.GetDetail:output_type -> usercenter.GetFormComponentDetailResponse
	10, // [10:15] is the sub-list for method output_type
	5,  // [5:10] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_form_component_proto_init() }
func file_form_component_proto_init() {
	if File_form_component_proto != nil {
		return
	}
	file_common_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_form_component_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FormComponentInfo); i {
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
		file_form_component_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FormComponentResource); i {
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
		file_form_component_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueryFormComponentRequest); i {
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
		file_form_component_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueryFormComponentResponse); i {
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
		file_form_component_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetFormComponentDetailResponse); i {
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
			RawDescriptor: file_form_component_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_form_component_proto_goTypes,
		DependencyIndexes: file_form_component_proto_depIdxs,
		MessageInfos:      file_form_component_proto_msgTypes,
	}.Build()
	File_form_component_proto = out.File
	file_form_component_proto_rawDesc = nil
	file_form_component_proto_goTypes = nil
	file_form_component_proto_depIdxs = nil
}
