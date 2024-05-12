// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.20.3
// source: project.proto

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

type ProjectInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id             string                  `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	TenantID       string                  `protobuf:"bytes,2,opt,name=tenantID,proto3" json:"tenantID"`
	Name           string                  `protobuf:"bytes,3,opt,name=name,proto3" json:"name"`
	FormCount      int32                   `protobuf:"varint,4,opt,name=formCount,proto3" json:"formCount"`
	PageCount      int32                   `protobuf:"varint,5,opt,name=pageCount,proto3" json:"pageCount"`
	Expired        string                  `protobuf:"bytes,6,opt,name=expired,proto3" json:"expired"`
	Description    string                  `protobuf:"bytes,7,opt,name=description,proto3" json:"description"`
	CellCount      int32                   `protobuf:"varint,8,opt,name=cellCount,proto3" json:"cellCount"`
	FormComponents []*ProjectFormComponent `protobuf:"bytes,9,rep,name=formComponents,proto3" json:"formComponents"`
	//系统必须要有的数据
	IsMust bool `protobuf:"varint,10,opt,name=isMust,proto3" json:"isMust"`
}

func (x *ProjectInfo) Reset() {
	*x = ProjectInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_project_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ProjectInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProjectInfo) ProtoMessage() {}

func (x *ProjectInfo) ProtoReflect() protoreflect.Message {
	mi := &file_project_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProjectInfo.ProtoReflect.Descriptor instead.
func (*ProjectInfo) Descriptor() ([]byte, []int) {
	return file_project_proto_rawDescGZIP(), []int{0}
}

func (x *ProjectInfo) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ProjectInfo) GetTenantID() string {
	if x != nil {
		return x.TenantID
	}
	return ""
}

func (x *ProjectInfo) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ProjectInfo) GetFormCount() int32 {
	if x != nil {
		return x.FormCount
	}
	return 0
}

func (x *ProjectInfo) GetPageCount() int32 {
	if x != nil {
		return x.PageCount
	}
	return 0
}

func (x *ProjectInfo) GetExpired() string {
	if x != nil {
		return x.Expired
	}
	return ""
}

func (x *ProjectInfo) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *ProjectInfo) GetCellCount() int32 {
	if x != nil {
		return x.CellCount
	}
	return 0
}

func (x *ProjectInfo) GetFormComponents() []*ProjectFormComponent {
	if x != nil {
		return x.FormComponents
	}
	return nil
}

func (x *ProjectInfo) GetIsMust() bool {
	if x != nil {
		return x.IsMust
	}
	return false
}

type ProjectFormComponent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id        string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	ProjectID string `protobuf:"bytes,2,opt,name=projectID,proto3" json:"projectID"`
	Name      string `protobuf:"bytes,3,opt,name=name,proto3" json:"name"`
}

func (x *ProjectFormComponent) Reset() {
	*x = ProjectFormComponent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_project_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ProjectFormComponent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProjectFormComponent) ProtoMessage() {}

func (x *ProjectFormComponent) ProtoReflect() protoreflect.Message {
	mi := &file_project_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProjectFormComponent.ProtoReflect.Descriptor instead.
func (*ProjectFormComponent) Descriptor() ([]byte, []int) {
	return file_project_proto_rawDescGZIP(), []int{1}
}

func (x *ProjectFormComponent) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *ProjectFormComponent) GetProjectID() string {
	if x != nil {
		return x.ProjectID
	}
	return ""
}

func (x *ProjectFormComponent) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type QueryProjectRequest struct {
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
	// @inject_tag: uri:"tenantID" form:"tenantID"
	TenantID string `protobuf:"bytes,5,opt,name=tenantID,proto3" json:"tenantID" uri:"tenantID" form:"tenantID"`
	// @inject_tag: uri:"name" form:"name"
	Name string `protobuf:"bytes,6,opt,name=name,proto3" json:"name" uri:"name" form:"name"`
	// @inject_tag: uri:"isMust" form:"isMust"
	IsMust bool `protobuf:"varint,7,opt,name=isMust,proto3" json:"isMust" uri:"isMust" form:"isMust"`
	// @inject_tag: uri:"sortConfig" form:"sortConfig"
	SortConfig string `protobuf:"bytes,8,opt,name=sortConfig,proto3" json:"sortConfig" uri:"sortConfig" form:"sortConfig"`
	// @inject_tag: uri:"ids" form:"ids"
	Ids []string `protobuf:"bytes,9,rep,name=ids,proto3" json:"ids" uri:"ids" form:"ids"`
}

func (x *QueryProjectRequest) Reset() {
	*x = QueryProjectRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_project_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueryProjectRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryProjectRequest) ProtoMessage() {}

func (x *QueryProjectRequest) ProtoReflect() protoreflect.Message {
	mi := &file_project_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryProjectRequest.ProtoReflect.Descriptor instead.
func (*QueryProjectRequest) Descriptor() ([]byte, []int) {
	return file_project_proto_rawDescGZIP(), []int{2}
}

func (x *QueryProjectRequest) GetPageIndex() int64 {
	if x != nil {
		return x.PageIndex
	}
	return 0
}

func (x *QueryProjectRequest) GetPageSize() int64 {
	if x != nil {
		return x.PageSize
	}
	return 0
}

func (x *QueryProjectRequest) GetOrderField() string {
	if x != nil {
		return x.OrderField
	}
	return ""
}

func (x *QueryProjectRequest) GetDesc() bool {
	if x != nil {
		return x.Desc
	}
	return false
}

func (x *QueryProjectRequest) GetTenantID() string {
	if x != nil {
		return x.TenantID
	}
	return ""
}

func (x *QueryProjectRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *QueryProjectRequest) GetIsMust() bool {
	if x != nil {
		return x.IsMust
	}
	return false
}

func (x *QueryProjectRequest) GetSortConfig() string {
	if x != nil {
		return x.SortConfig
	}
	return ""
}

func (x *QueryProjectRequest) GetIds() []string {
	if x != nil {
		return x.Ids
	}
	return nil
}

type QueryProjectResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code           `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string         `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    []*ProjectInfo `protobuf:"bytes,3,rep,name=data,proto3" json:"data"`
	Pages   int64          `protobuf:"varint,4,opt,name=pages,proto3" json:"pages"`
	Records int64          `protobuf:"varint,5,opt,name=records,proto3" json:"records"`
	Total   int64          `protobuf:"varint,6,opt,name=total,proto3" json:"total"`
}

func (x *QueryProjectResponse) Reset() {
	*x = QueryProjectResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_project_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueryProjectResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryProjectResponse) ProtoMessage() {}

func (x *QueryProjectResponse) ProtoReflect() protoreflect.Message {
	mi := &file_project_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryProjectResponse.ProtoReflect.Descriptor instead.
func (*QueryProjectResponse) Descriptor() ([]byte, []int) {
	return file_project_proto_rawDescGZIP(), []int{3}
}

func (x *QueryProjectResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *QueryProjectResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *QueryProjectResponse) GetData() []*ProjectInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *QueryProjectResponse) GetPages() int64 {
	if x != nil {
		return x.Pages
	}
	return 0
}

func (x *QueryProjectResponse) GetRecords() int64 {
	if x != nil {
		return x.Records
	}
	return 0
}

func (x *QueryProjectResponse) GetTotal() int64 {
	if x != nil {
		return x.Total
	}
	return 0
}

type GetProjectDetailResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code         `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string       `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    *ProjectInfo `protobuf:"bytes,3,opt,name=data,proto3" json:"data"`
}

func (x *GetProjectDetailResponse) Reset() {
	*x = GetProjectDetailResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_project_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetProjectDetailResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetProjectDetailResponse) ProtoMessage() {}

func (x *GetProjectDetailResponse) ProtoReflect() protoreflect.Message {
	mi := &file_project_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetProjectDetailResponse.ProtoReflect.Descriptor instead.
func (*GetProjectDetailResponse) Descriptor() ([]byte, []int) {
	return file_project_proto_rawDescGZIP(), []int{4}
}

func (x *GetProjectDetailResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *GetProjectDetailResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *GetProjectDetailResponse) GetData() *ProjectInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_project_proto protoreflect.FileDescriptor

var file_project_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x0a, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x1a, 0x0c, 0x63, 0x6f, 0x6d,
	0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xc5, 0x02, 0x0a, 0x0b, 0x50, 0x72,
	0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x74, 0x65, 0x6e,
	0x61, 0x6e, 0x74, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x65, 0x6e,
	0x61, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x66, 0x6f, 0x72,
	0x6d, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x66, 0x6f,
	0x72, 0x6d, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x61, 0x67, 0x65, 0x43,
	0x6f, 0x75, 0x6e, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x70, 0x61, 0x67, 0x65,
	0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x64,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x65, 0x78, 0x70, 0x69, 0x72, 0x65, 0x64, 0x12,
	0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x07,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x12, 0x1c, 0x0a, 0x09, 0x63, 0x65, 0x6c, 0x6c, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x08,
	0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x63, 0x65, 0x6c, 0x6c, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12,
	0x48, 0x0a, 0x0e, 0x66, 0x6f, 0x72, 0x6d, 0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74,
	0x73, 0x18, 0x09, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x20, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x46, 0x6f, 0x72, 0x6d,
	0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x52, 0x0e, 0x66, 0x6f, 0x72, 0x6d, 0x43,
	0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x69, 0x73, 0x4d,
	0x75, 0x73, 0x74, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x08, 0x52, 0x06, 0x69, 0x73, 0x4d, 0x75, 0x73,
	0x74, 0x22, 0x58, 0x0a, 0x14, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x46, 0x6f, 0x72, 0x6d,
	0x43, 0x6f, 0x6d, 0x70, 0x6f, 0x6e, 0x65, 0x6e, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x72, 0x6f,
	0x6a, 0x65, 0x63, 0x74, 0x49, 0x44, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x70, 0x72,
	0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x44, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0xfd, 0x01, 0x0a, 0x13,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x61, 0x67, 0x65, 0x49, 0x6e, 0x64, 0x65, 0x78,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x70, 0x61, 0x67, 0x65, 0x49, 0x6e, 0x64, 0x65,
	0x78, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x61, 0x67, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x08, 0x70, 0x61, 0x67, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x12, 0x1e, 0x0a,
	0x0a, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0a, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12, 0x12, 0x0a,
	0x04, 0x64, 0x65, 0x73, 0x63, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x04, 0x64, 0x65, 0x73,
	0x63, 0x12, 0x1a, 0x0a, 0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x12, 0x0a,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x12, 0x16, 0x0a, 0x06, 0x69, 0x73, 0x4d, 0x75, 0x73, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x06, 0x69, 0x73, 0x4d, 0x75, 0x73, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x6f, 0x72,
	0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x73,
	0x6f, 0x72, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x10, 0x0a, 0x03, 0x69, 0x64, 0x73,
	0x18, 0x09, 0x20, 0x03, 0x28, 0x09, 0x52, 0x03, 0x69, 0x64, 0x73, 0x22, 0xc9, 0x01, 0x0a, 0x14,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x24, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x10, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x43, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x12, 0x2b, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x17, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x12, 0x14, 0x0a, 0x05, 0x70, 0x61, 0x67, 0x65, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x05, 0x70, 0x61, 0x67, 0x65, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x72, 0x65, 0x63, 0x6f, 0x72,
	0x64, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64,
	0x73, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x05, 0x74, 0x6f, 0x74, 0x61, 0x6c, 0x22, 0x87, 0x01, 0x0a, 0x18, 0x47, 0x65, 0x74, 0x50,
	0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x24, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x10, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x43, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x12, 0x2b, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x17, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x32, 0xb8, 0x03, 0x0a, 0x07, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x12, 0x3c, 0x0a,
	0x03, 0x41, 0x64, 0x64, 0x12, 0x17, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65,
	0x72, 0x2e, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x1a, 0x2e,
	0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f,
	0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3f, 0x0a, 0x06, 0x55,
	0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x17, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x65, 0x72, 0x2e, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x1a,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d,
	0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3e, 0x0a, 0x06,
	0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x12, 0x16, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e,
	0x74, 0x65, 0x72, 0x2e, 0x44, 0x65, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d,
	0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4c, 0x0a, 0x05,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x12, 0x1f, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x65, 0x72, 0x2e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x20, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e,
	0x74, 0x65, 0x72, 0x2e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x51, 0x0a, 0x09, 0x47, 0x65,
	0x74, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x12, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x47, 0x65, 0x74, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x24, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x65, 0x72, 0x2e, 0x47, 0x65, 0x74, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x44, 0x65, 0x74,
	0x61, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4d, 0x0a,
	0x06, 0x45, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x1f, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x45, 0x78, 0x70, 0x6f, 0x72,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x20, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63,
	0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x45, 0x78, 0x70, 0x6f,
	0x72, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x41, 0x0a, 0x13,
	0x63, 0x6e, 0x2e, 0x61, 0x74, 0x61, 0x6c, 0x69, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e,
	0x74, 0x65, 0x72, 0x42, 0x0c, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x50, 0x72, 0x6f, 0x74,
	0x6f, 0x50, 0x01, 0x5a, 0x0d, 0x2e, 0x2f, 0x3b, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74,
	0x65, 0x72, 0xa2, 0x02, 0x0a, 0x50, 0x52, 0x4f, 0x4a, 0x45, 0x43, 0x54, 0x53, 0x52, 0x56, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_project_proto_rawDescOnce sync.Once
	file_project_proto_rawDescData = file_project_proto_rawDesc
)

func file_project_proto_rawDescGZIP() []byte {
	file_project_proto_rawDescOnce.Do(func() {
		file_project_proto_rawDescData = protoimpl.X.CompressGZIP(file_project_proto_rawDescData)
	})
	return file_project_proto_rawDescData
}

var file_project_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_project_proto_goTypes = []interface{}{
	(*ProjectInfo)(nil),              // 0: usercenter.ProjectInfo
	(*ProjectFormComponent)(nil),     // 1: usercenter.ProjectFormComponent
	(*QueryProjectRequest)(nil),      // 2: usercenter.QueryProjectRequest
	(*QueryProjectResponse)(nil),     // 3: usercenter.QueryProjectResponse
	(*GetProjectDetailResponse)(nil), // 4: usercenter.GetProjectDetailResponse
	(Code)(0),                        // 5: usercenter.Code
	(*DelRequest)(nil),               // 6: usercenter.DelRequest
	(*GetDetailRequest)(nil),         // 7: usercenter.GetDetailRequest
	(*CommonExportRequest)(nil),      // 8: usercenter.CommonExportRequest
	(*CommonResponse)(nil),           // 9: usercenter.CommonResponse
	(*CommonExportResponse)(nil),     // 10: usercenter.CommonExportResponse
}
var file_project_proto_depIdxs = []int32{
	1,  // 0: usercenter.ProjectInfo.formComponents:type_name -> usercenter.ProjectFormComponent
	5,  // 1: usercenter.QueryProjectResponse.code:type_name -> usercenter.Code
	0,  // 2: usercenter.QueryProjectResponse.data:type_name -> usercenter.ProjectInfo
	5,  // 3: usercenter.GetProjectDetailResponse.code:type_name -> usercenter.Code
	0,  // 4: usercenter.GetProjectDetailResponse.data:type_name -> usercenter.ProjectInfo
	0,  // 5: usercenter.Project.Add:input_type -> usercenter.ProjectInfo
	0,  // 6: usercenter.Project.Update:input_type -> usercenter.ProjectInfo
	6,  // 7: usercenter.Project.Delete:input_type -> usercenter.DelRequest
	2,  // 8: usercenter.Project.Query:input_type -> usercenter.QueryProjectRequest
	7,  // 9: usercenter.Project.GetDetail:input_type -> usercenter.GetDetailRequest
	8,  // 10: usercenter.Project.Export:input_type -> usercenter.CommonExportRequest
	9,  // 11: usercenter.Project.Add:output_type -> usercenter.CommonResponse
	9,  // 12: usercenter.Project.Update:output_type -> usercenter.CommonResponse
	9,  // 13: usercenter.Project.Delete:output_type -> usercenter.CommonResponse
	3,  // 14: usercenter.Project.Query:output_type -> usercenter.QueryProjectResponse
	4,  // 15: usercenter.Project.GetDetail:output_type -> usercenter.GetProjectDetailResponse
	10, // 16: usercenter.Project.Export:output_type -> usercenter.CommonExportResponse
	11, // [11:17] is the sub-list for method output_type
	5,  // [5:11] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_project_proto_init() }
func file_project_proto_init() {
	if File_project_proto != nil {
		return
	}
	file_common_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_project_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ProjectInfo); i {
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
		file_project_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ProjectFormComponent); i {
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
		file_project_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueryProjectRequest); i {
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
		file_project_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueryProjectResponse); i {
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
		file_project_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetProjectDetailResponse); i {
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
			RawDescriptor: file_project_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_project_proto_goTypes,
		DependencyIndexes: file_project_proto_depIdxs,
		MessageInfos:      file_project_proto_msgTypes,
	}.Build()
	File_project_proto = out.File
	file_project_proto_rawDesc = nil
	file_project_proto_goTypes = nil
	file_project_proto_depIdxs = nil
}