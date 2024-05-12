// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.20.3
// source: dictionaries.proto

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

type DictionariesInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	//Description
	Description string `protobuf:"bytes,2,opt,name=description,proto3" json:"description"`
	//Name
	Name string `protobuf:"bytes,3,opt,name=name,proto3" json:"name"`
	//Type
	Type string `protobuf:"bytes,4,opt,name=type,proto3" json:"type"`
	//Value
	Value string `protobuf:"bytes,5,opt,name=value,proto3" json:"value"`
	//租户ID
	ProjectID string `protobuf:"bytes,6,opt,name=projectID,proto3" json:"projectID"`
	TenantID  string `protobuf:"bytes,7,opt,name=tenantID,proto3" json:"tenantID"`
	//系统必须要有的数据
	IsMust bool `protobuf:"varint,8,opt,name=isMust,proto3" json:"isMust"`
}

func (x *DictionariesInfo) Reset() {
	*x = DictionariesInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dictionaries_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DictionariesInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DictionariesInfo) ProtoMessage() {}

func (x *DictionariesInfo) ProtoReflect() protoreflect.Message {
	mi := &file_dictionaries_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DictionariesInfo.ProtoReflect.Descriptor instead.
func (*DictionariesInfo) Descriptor() ([]byte, []int) {
	return file_dictionaries_proto_rawDescGZIP(), []int{0}
}

func (x *DictionariesInfo) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *DictionariesInfo) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *DictionariesInfo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *DictionariesInfo) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *DictionariesInfo) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

func (x *DictionariesInfo) GetProjectID() string {
	if x != nil {
		return x.ProjectID
	}
	return ""
}

func (x *DictionariesInfo) GetTenantID() string {
	if x != nil {
		return x.TenantID
	}
	return ""
}

func (x *DictionariesInfo) GetIsMust() bool {
	if x != nil {
		return x.IsMust
	}
	return false
}

type QueryDictionariesRequest struct {
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
	//租户ID
	// @inject_tag: uri:"tenantID" form:"tenantID"
	TenantID string `protobuf:"bytes,9,opt,name=tenantID,proto3" json:"tenantID" uri:"tenantID" form:"tenantID"`
	// @inject_tag: uri:"isMust" form:"isMust"
	IsMust bool `protobuf:"varint,10,opt,name=isMust,proto3" json:"isMust" uri:"isMust" form:"isMust"`
	// @inject_tag: uri:"sortConfig" form:"sortConfig"
	SortConfig string `protobuf:"bytes,11,opt,name=sortConfig,proto3" json:"sortConfig" uri:"sortConfig" form:"sortConfig"`
}

func (x *QueryDictionariesRequest) Reset() {
	*x = QueryDictionariesRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dictionaries_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueryDictionariesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryDictionariesRequest) ProtoMessage() {}

func (x *QueryDictionariesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_dictionaries_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryDictionariesRequest.ProtoReflect.Descriptor instead.
func (*QueryDictionariesRequest) Descriptor() ([]byte, []int) {
	return file_dictionaries_proto_rawDescGZIP(), []int{1}
}

func (x *QueryDictionariesRequest) GetPageIndex() int64 {
	if x != nil {
		return x.PageIndex
	}
	return 0
}

func (x *QueryDictionariesRequest) GetPageSize() int64 {
	if x != nil {
		return x.PageSize
	}
	return 0
}

func (x *QueryDictionariesRequest) GetOrderField() string {
	if x != nil {
		return x.OrderField
	}
	return ""
}

func (x *QueryDictionariesRequest) GetDesc() bool {
	if x != nil {
		return x.Desc
	}
	return false
}

func (x *QueryDictionariesRequest) GetTenantID() string {
	if x != nil {
		return x.TenantID
	}
	return ""
}

func (x *QueryDictionariesRequest) GetIsMust() bool {
	if x != nil {
		return x.IsMust
	}
	return false
}

func (x *QueryDictionariesRequest) GetSortConfig() string {
	if x != nil {
		return x.SortConfig
	}
	return ""
}

type QueryDictionariesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code                `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string              `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    []*DictionariesInfo `protobuf:"bytes,3,rep,name=data,proto3" json:"data"`
	Pages   int64               `protobuf:"varint,4,opt,name=pages,proto3" json:"pages"`
	Records int64               `protobuf:"varint,5,opt,name=records,proto3" json:"records"`
	Total   int64               `protobuf:"varint,6,opt,name=total,proto3" json:"total"`
}

func (x *QueryDictionariesResponse) Reset() {
	*x = QueryDictionariesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dictionaries_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueryDictionariesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryDictionariesResponse) ProtoMessage() {}

func (x *QueryDictionariesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_dictionaries_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryDictionariesResponse.ProtoReflect.Descriptor instead.
func (*QueryDictionariesResponse) Descriptor() ([]byte, []int) {
	return file_dictionaries_proto_rawDescGZIP(), []int{2}
}

func (x *QueryDictionariesResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *QueryDictionariesResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *QueryDictionariesResponse) GetData() []*DictionariesInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *QueryDictionariesResponse) GetPages() int64 {
	if x != nil {
		return x.Pages
	}
	return 0
}

func (x *QueryDictionariesResponse) GetRecords() int64 {
	if x != nil {
		return x.Records
	}
	return 0
}

func (x *QueryDictionariesResponse) GetTotal() int64 {
	if x != nil {
		return x.Total
	}
	return 0
}

type GetAllDictionariesResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code                `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string              `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    []*DictionariesInfo `protobuf:"bytes,3,rep,name=data,proto3" json:"data"`
}

func (x *GetAllDictionariesResponse) Reset() {
	*x = GetAllDictionariesResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dictionaries_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetAllDictionariesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetAllDictionariesResponse) ProtoMessage() {}

func (x *GetAllDictionariesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_dictionaries_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetAllDictionariesResponse.ProtoReflect.Descriptor instead.
func (*GetAllDictionariesResponse) Descriptor() ([]byte, []int) {
	return file_dictionaries_proto_rawDescGZIP(), []int{3}
}

func (x *GetAllDictionariesResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *GetAllDictionariesResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *GetAllDictionariesResponse) GetData() []*DictionariesInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

type GetDictionariesDetailResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code              `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string            `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    *DictionariesInfo `protobuf:"bytes,3,opt,name=data,proto3" json:"data"`
}

func (x *GetDictionariesDetailResponse) Reset() {
	*x = GetDictionariesDetailResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_dictionaries_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetDictionariesDetailResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetDictionariesDetailResponse) ProtoMessage() {}

func (x *GetDictionariesDetailResponse) ProtoReflect() protoreflect.Message {
	mi := &file_dictionaries_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetDictionariesDetailResponse.ProtoReflect.Descriptor instead.
func (*GetDictionariesDetailResponse) Descriptor() ([]byte, []int) {
	return file_dictionaries_proto_rawDescGZIP(), []int{4}
}

func (x *GetDictionariesDetailResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *GetDictionariesDetailResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *GetDictionariesDetailResponse) GetData() *DictionariesInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_dictionaries_proto protoreflect.FileDescriptor

var file_dictionaries_proto_rawDesc = []byte{
	0x0a, 0x12, 0x64, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0a, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72,
	0x1a, 0x0c, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xd4,
	0x01, 0x0a, 0x10, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x49,
	0x6e, 0x66, 0x6f, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x02, 0x69, 0x64, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69,
	0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70,
	0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x14, 0x0a,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x44,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49,
	0x44, 0x12, 0x1a, 0x0a, 0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x18, 0x07, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x16, 0x0a,
	0x06, 0x69, 0x73, 0x4d, 0x75, 0x73, 0x74, 0x18, 0x08, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06, 0x69,
	0x73, 0x4d, 0x75, 0x73, 0x74, 0x22, 0xdc, 0x01, 0x0a, 0x18, 0x51, 0x75, 0x65, 0x72, 0x79, 0x44,
	0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x61, 0x67, 0x65, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x70, 0x61, 0x67, 0x65, 0x49, 0x6e, 0x64, 0x65, 0x78,
	0x12, 0x1a, 0x0a, 0x08, 0x70, 0x61, 0x67, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x08, 0x70, 0x61, 0x67, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x12, 0x1e, 0x0a, 0x0a,
	0x6f, 0x72, 0x64, 0x65, 0x72, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0a, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12, 0x12, 0x0a, 0x04,
	0x64, 0x65, 0x73, 0x63, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x04, 0x64, 0x65, 0x73, 0x63,
	0x12, 0x1a, 0x0a, 0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x18, 0x09, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x16, 0x0a, 0x06,
	0x69, 0x73, 0x4d, 0x75, 0x73, 0x74, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06, 0x69, 0x73,
	0x4d, 0x75, 0x73, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x6f, 0x72, 0x74, 0x43, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x73, 0x6f, 0x72, 0x74, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x22, 0xd3, 0x01, 0x0a, 0x19, 0x51, 0x75, 0x65, 0x72, 0x79, 0x44, 0x69,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x24, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x10, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f,
	0x64, 0x65, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x12, 0x30, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x44, 0x69,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04,
	0x64, 0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x70, 0x61, 0x67, 0x65, 0x73, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x05, 0x70, 0x61, 0x67, 0x65, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x72, 0x65,
	0x63, 0x6f, 0x72, 0x64, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x72, 0x65, 0x63,
	0x6f, 0x72, 0x64, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x18, 0x06, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x22, 0x8e, 0x01, 0x0a, 0x1a, 0x47,
	0x65, 0x74, 0x41, 0x6c, 0x6c, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65,
	0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x24, 0x0a, 0x04, 0x63, 0x6f, 0x64,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x10, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12,
	0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x30, 0x0a, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65,
	0x73, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x91, 0x01, 0x0a, 0x1d,
	0x47, 0x65, 0x74, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x44,
	0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x24, 0x0a,
	0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x10, 0x2e, 0x75, 0x73,
	0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x63,
	0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x30, 0x0a,
	0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x75, 0x73,
	0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e,
	0x61, 0x72, 0x69, 0x65, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x32,
	0xa5, 0x04, 0x0a, 0x0c, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73,
	0x12, 0x41, 0x0a, 0x03, 0x41, 0x64, 0x64, 0x12, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65,
	0x73, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x1a, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x44, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x1c, 0x2e,
	0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x44, 0x69, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x1a, 0x2e, 0x75, 0x73,
	0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3e, 0x0a, 0x06, 0x44, 0x65, 0x6c,
	0x65, 0x74, 0x65, 0x12, 0x16, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72,
	0x2e, 0x44, 0x65, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x75, 0x73,
	0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x56, 0x0a, 0x05, 0x51, 0x75, 0x65,
	0x72, 0x79, 0x12, 0x24, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65,
	0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x25, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63,
	0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x44, 0x69, 0x63, 0x74, 0x69,
	0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x4d, 0x0a, 0x06, 0x47, 0x65, 0x74, 0x41, 0x6c, 0x6c, 0x12, 0x19, 0x2e, 0x75, 0x73,
	0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x47, 0x65, 0x74, 0x41, 0x6c, 0x6c, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x26, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e,
	0x74, 0x65, 0x72, 0x2e, 0x47, 0x65, 0x74, 0x41, 0x6c, 0x6c, 0x44, 0x69, 0x63, 0x74, 0x69, 0x6f,
	0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x12, 0x56, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x12, 0x1c, 0x2e,
	0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x47, 0x65, 0x74, 0x44, 0x65,
	0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x29, 0x2e, 0x75, 0x73,
	0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x47, 0x65, 0x74, 0x44, 0x69, 0x63, 0x74,
	0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4d, 0x0a, 0x06, 0x45, 0x78, 0x70, 0x6f,
	0x72, 0x74, 0x12, 0x1f, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x45, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x20, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72,
	0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x45, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x4a, 0x0a, 0x13, 0x63, 0x6e, 0x2e, 0x61, 0x74,
	0x61, 0x6c, 0x69, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x42, 0x11,
	0x44, 0x69, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x72, 0x69, 0x65, 0x73, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x50, 0x01, 0x5a, 0x0d, 0x2e, 0x2f, 0x3b, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x65, 0x72, 0xa2, 0x02, 0x0e, 0x44, 0x49, 0x43, 0x54, 0x49, 0x4f, 0x4e, 0x41, 0x52, 0x49, 0x45,
	0x53, 0x52, 0x56, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_dictionaries_proto_rawDescOnce sync.Once
	file_dictionaries_proto_rawDescData = file_dictionaries_proto_rawDesc
)

func file_dictionaries_proto_rawDescGZIP() []byte {
	file_dictionaries_proto_rawDescOnce.Do(func() {
		file_dictionaries_proto_rawDescData = protoimpl.X.CompressGZIP(file_dictionaries_proto_rawDescData)
	})
	return file_dictionaries_proto_rawDescData
}

var file_dictionaries_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_dictionaries_proto_goTypes = []interface{}{
	(*DictionariesInfo)(nil),              // 0: usercenter.DictionariesInfo
	(*QueryDictionariesRequest)(nil),      // 1: usercenter.QueryDictionariesRequest
	(*QueryDictionariesResponse)(nil),     // 2: usercenter.QueryDictionariesResponse
	(*GetAllDictionariesResponse)(nil),    // 3: usercenter.GetAllDictionariesResponse
	(*GetDictionariesDetailResponse)(nil), // 4: usercenter.GetDictionariesDetailResponse
	(Code)(0),                             // 5: usercenter.Code
	(*DelRequest)(nil),                    // 6: usercenter.DelRequest
	(*GetAllRequest)(nil),                 // 7: usercenter.GetAllRequest
	(*GetDetailRequest)(nil),              // 8: usercenter.GetDetailRequest
	(*CommonExportRequest)(nil),           // 9: usercenter.CommonExportRequest
	(*CommonResponse)(nil),                // 10: usercenter.CommonResponse
	(*CommonExportResponse)(nil),          // 11: usercenter.CommonExportResponse
}
var file_dictionaries_proto_depIdxs = []int32{
	5,  // 0: usercenter.QueryDictionariesResponse.code:type_name -> usercenter.Code
	0,  // 1: usercenter.QueryDictionariesResponse.data:type_name -> usercenter.DictionariesInfo
	5,  // 2: usercenter.GetAllDictionariesResponse.code:type_name -> usercenter.Code
	0,  // 3: usercenter.GetAllDictionariesResponse.data:type_name -> usercenter.DictionariesInfo
	5,  // 4: usercenter.GetDictionariesDetailResponse.code:type_name -> usercenter.Code
	0,  // 5: usercenter.GetDictionariesDetailResponse.data:type_name -> usercenter.DictionariesInfo
	0,  // 6: usercenter.Dictionaries.Add:input_type -> usercenter.DictionariesInfo
	0,  // 7: usercenter.Dictionaries.Update:input_type -> usercenter.DictionariesInfo
	6,  // 8: usercenter.Dictionaries.Delete:input_type -> usercenter.DelRequest
	1,  // 9: usercenter.Dictionaries.Query:input_type -> usercenter.QueryDictionariesRequest
	7,  // 10: usercenter.Dictionaries.GetAll:input_type -> usercenter.GetAllRequest
	8,  // 11: usercenter.Dictionaries.GetDetail:input_type -> usercenter.GetDetailRequest
	9,  // 12: usercenter.Dictionaries.Export:input_type -> usercenter.CommonExportRequest
	10, // 13: usercenter.Dictionaries.Add:output_type -> usercenter.CommonResponse
	10, // 14: usercenter.Dictionaries.Update:output_type -> usercenter.CommonResponse
	10, // 15: usercenter.Dictionaries.Delete:output_type -> usercenter.CommonResponse
	2,  // 16: usercenter.Dictionaries.Query:output_type -> usercenter.QueryDictionariesResponse
	3,  // 17: usercenter.Dictionaries.GetAll:output_type -> usercenter.GetAllDictionariesResponse
	4,  // 18: usercenter.Dictionaries.GetDetail:output_type -> usercenter.GetDictionariesDetailResponse
	11, // 19: usercenter.Dictionaries.Export:output_type -> usercenter.CommonExportResponse
	13, // [13:20] is the sub-list for method output_type
	6,  // [6:13] is the sub-list for method input_type
	6,  // [6:6] is the sub-list for extension type_name
	6,  // [6:6] is the sub-list for extension extendee
	0,  // [0:6] is the sub-list for field type_name
}

func init() { file_dictionaries_proto_init() }
func file_dictionaries_proto_init() {
	if File_dictionaries_proto != nil {
		return
	}
	file_common_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_dictionaries_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DictionariesInfo); i {
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
		file_dictionaries_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueryDictionariesRequest); i {
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
		file_dictionaries_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueryDictionariesResponse); i {
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
		file_dictionaries_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetAllDictionariesResponse); i {
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
		file_dictionaries_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetDictionariesDetailResponse); i {
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
			RawDescriptor: file_dictionaries_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_dictionaries_proto_goTypes,
		DependencyIndexes: file_dictionaries_proto_depIdxs,
		MessageInfos:      file_dictionaries_proto_msgTypes,
	}.Build()
	File_dictionaries_proto = out.File
	file_dictionaries_proto_rawDesc = nil
	file_dictionaries_proto_goTypes = nil
	file_dictionaries_proto_depIdxs = nil
}
