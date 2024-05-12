// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.20.3
// source: wechat_config.proto

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

type WechatConfigInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id"`
	//APP ID
	AppID string `protobuf:"bytes,2,opt,name=appID,proto3" json:"appID"`
	//APP名称
	AppName string `protobuf:"bytes,3,opt,name=appName,proto3" json:"appName"`
	//秘钥
	Secret string `protobuf:"bytes,4,opt,name=secret,proto3" json:"secret"`
	//租户ID
	TenantID string `protobuf:"bytes,5,opt,name=tenantID,proto3" json:"tenantID"`
	//类型 1-微信小程序 2-微信公众号 3-微信APP应用 4-微信网站应用
	AppType int32 `protobuf:"varint,6,opt,name=appType,proto3" json:"appType"`
	//默认角色 用户通过微信注册时赋予默认角色
	DefaultRoleID string `protobuf:"bytes,7,opt,name=defaultRoleID,proto3" json:"defaultRoleID"`
	//重定向URL
	RedirectUrl string `protobuf:"bytes,8,opt,name=redirectUrl,proto3" json:"redirectUrl"`
	//Token
	Token string `protobuf:"bytes,9,opt,name=token,proto3" json:"token"`
	//消息加解密密钥
	EncodingAESKey string `protobuf:"bytes,10,opt,name=encodingAESKey,proto3" json:"encodingAESKey"`
	//消息加解密方式,1-明文模式,2-兼容模式,3-安全模式（推荐）
	EncodingMethod int32 `protobuf:"varint,11,opt,name=encodingMethod,proto3" json:"encodingMethod"`
	//显示名称
	DisplayName string `protobuf:"bytes,12,opt,name=displayName,proto3" json:"displayName"`
	ProjectID   string `protobuf:"bytes,13,opt,name=projectID,proto3" json:"projectID"`
	//系统必须要有的数据
	IsMust bool `protobuf:"varint,14,opt,name=isMust,proto3" json:"isMust"`
}

func (x *WechatConfigInfo) Reset() {
	*x = WechatConfigInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_wechat_config_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WechatConfigInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WechatConfigInfo) ProtoMessage() {}

func (x *WechatConfigInfo) ProtoReflect() protoreflect.Message {
	mi := &file_wechat_config_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WechatConfigInfo.ProtoReflect.Descriptor instead.
func (*WechatConfigInfo) Descriptor() ([]byte, []int) {
	return file_wechat_config_proto_rawDescGZIP(), []int{0}
}

func (x *WechatConfigInfo) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *WechatConfigInfo) GetAppID() string {
	if x != nil {
		return x.AppID
	}
	return ""
}

func (x *WechatConfigInfo) GetAppName() string {
	if x != nil {
		return x.AppName
	}
	return ""
}

func (x *WechatConfigInfo) GetSecret() string {
	if x != nil {
		return x.Secret
	}
	return ""
}

func (x *WechatConfigInfo) GetTenantID() string {
	if x != nil {
		return x.TenantID
	}
	return ""
}

func (x *WechatConfigInfo) GetAppType() int32 {
	if x != nil {
		return x.AppType
	}
	return 0
}

func (x *WechatConfigInfo) GetDefaultRoleID() string {
	if x != nil {
		return x.DefaultRoleID
	}
	return ""
}

func (x *WechatConfigInfo) GetRedirectUrl() string {
	if x != nil {
		return x.RedirectUrl
	}
	return ""
}

func (x *WechatConfigInfo) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

func (x *WechatConfigInfo) GetEncodingAESKey() string {
	if x != nil {
		return x.EncodingAESKey
	}
	return ""
}

func (x *WechatConfigInfo) GetEncodingMethod() int32 {
	if x != nil {
		return x.EncodingMethod
	}
	return 0
}

func (x *WechatConfigInfo) GetDisplayName() string {
	if x != nil {
		return x.DisplayName
	}
	return ""
}

func (x *WechatConfigInfo) GetProjectID() string {
	if x != nil {
		return x.ProjectID
	}
	return ""
}

func (x *WechatConfigInfo) GetIsMust() bool {
	if x != nil {
		return x.IsMust
	}
	return false
}

type QueryWechatConfigRequest struct {
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
	//APP名称
	// @inject_tag: uri:"appName" form:"appName"
	AppName string `protobuf:"bytes,6,opt,name=appName,proto3" json:"appName" uri:"appName" form:"appName"`
	//租户ID
	// @inject_tag: uri:"tenantID" form:"tenantID"
	TenantID string `protobuf:"bytes,8,opt,name=tenantID,proto3" json:"tenantID" uri:"tenantID" form:"tenantID"`
	//类型 1-微信小程序 2-微信公众号 3-微信APP应用 4-微信网站应用
	// @inject_tag: uri:"appType" form:"appType"
	AppType int32 `protobuf:"varint,9,opt,name=appType,proto3" json:"appType" uri:"appType" form:"appType"`
	// @inject_tag: uri:"isMust" form:"isMust"
	IsMust bool `protobuf:"varint,10,opt,name=isMust,proto3" json:"isMust" uri:"isMust" form:"isMust"`
	// @inject_tag: uri:"sortConfig" form:"sortConfig"
	SortConfig string `protobuf:"bytes,11,opt,name=sortConfig,proto3" json:"sortConfig" uri:"sortConfig" form:"sortConfig"`
}

func (x *QueryWechatConfigRequest) Reset() {
	*x = QueryWechatConfigRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_wechat_config_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueryWechatConfigRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryWechatConfigRequest) ProtoMessage() {}

func (x *QueryWechatConfigRequest) ProtoReflect() protoreflect.Message {
	mi := &file_wechat_config_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryWechatConfigRequest.ProtoReflect.Descriptor instead.
func (*QueryWechatConfigRequest) Descriptor() ([]byte, []int) {
	return file_wechat_config_proto_rawDescGZIP(), []int{1}
}

func (x *QueryWechatConfigRequest) GetPageIndex() int64 {
	if x != nil {
		return x.PageIndex
	}
	return 0
}

func (x *QueryWechatConfigRequest) GetPageSize() int64 {
	if x != nil {
		return x.PageSize
	}
	return 0
}

func (x *QueryWechatConfigRequest) GetOrderField() string {
	if x != nil {
		return x.OrderField
	}
	return ""
}

func (x *QueryWechatConfigRequest) GetDesc() bool {
	if x != nil {
		return x.Desc
	}
	return false
}

func (x *QueryWechatConfigRequest) GetAppName() string {
	if x != nil {
		return x.AppName
	}
	return ""
}

func (x *QueryWechatConfigRequest) GetTenantID() string {
	if x != nil {
		return x.TenantID
	}
	return ""
}

func (x *QueryWechatConfigRequest) GetAppType() int32 {
	if x != nil {
		return x.AppType
	}
	return 0
}

func (x *QueryWechatConfigRequest) GetIsMust() bool {
	if x != nil {
		return x.IsMust
	}
	return false
}

func (x *QueryWechatConfigRequest) GetSortConfig() string {
	if x != nil {
		return x.SortConfig
	}
	return ""
}

type QueryWechatConfigResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code                `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string              `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    []*WechatConfigInfo `protobuf:"bytes,3,rep,name=data,proto3" json:"data"`
	Pages   int64               `protobuf:"varint,4,opt,name=pages,proto3" json:"pages"`
	Records int64               `protobuf:"varint,5,opt,name=records,proto3" json:"records"`
	Total   int64               `protobuf:"varint,6,opt,name=total,proto3" json:"total"`
}

func (x *QueryWechatConfigResponse) Reset() {
	*x = QueryWechatConfigResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_wechat_config_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *QueryWechatConfigResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*QueryWechatConfigResponse) ProtoMessage() {}

func (x *QueryWechatConfigResponse) ProtoReflect() protoreflect.Message {
	mi := &file_wechat_config_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use QueryWechatConfigResponse.ProtoReflect.Descriptor instead.
func (*QueryWechatConfigResponse) Descriptor() ([]byte, []int) {
	return file_wechat_config_proto_rawDescGZIP(), []int{2}
}

func (x *QueryWechatConfigResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *QueryWechatConfigResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *QueryWechatConfigResponse) GetData() []*WechatConfigInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *QueryWechatConfigResponse) GetPages() int64 {
	if x != nil {
		return x.Pages
	}
	return 0
}

func (x *QueryWechatConfigResponse) GetRecords() int64 {
	if x != nil {
		return x.Records
	}
	return 0
}

func (x *QueryWechatConfigResponse) GetTotal() int64 {
	if x != nil {
		return x.Total
	}
	return 0
}

type GetAllWechatConfigResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code                `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string              `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    []*WechatConfigInfo `protobuf:"bytes,3,rep,name=data,proto3" json:"data"`
}

func (x *GetAllWechatConfigResponse) Reset() {
	*x = GetAllWechatConfigResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_wechat_config_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetAllWechatConfigResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetAllWechatConfigResponse) ProtoMessage() {}

func (x *GetAllWechatConfigResponse) ProtoReflect() protoreflect.Message {
	mi := &file_wechat_config_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetAllWechatConfigResponse.ProtoReflect.Descriptor instead.
func (*GetAllWechatConfigResponse) Descriptor() ([]byte, []int) {
	return file_wechat_config_proto_rawDescGZIP(), []int{3}
}

func (x *GetAllWechatConfigResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *GetAllWechatConfigResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *GetAllWechatConfigResponse) GetData() []*WechatConfigInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

type GetWechatConfigDetailResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    Code              `protobuf:"varint,1,opt,name=code,proto3,enum=usercenter.Code" json:"code"`
	Message string            `protobuf:"bytes,2,opt,name=message,proto3" json:"message"`
	Data    *WechatConfigInfo `protobuf:"bytes,3,opt,name=data,proto3" json:"data"`
}

func (x *GetWechatConfigDetailResponse) Reset() {
	*x = GetWechatConfigDetailResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_wechat_config_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetWechatConfigDetailResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetWechatConfigDetailResponse) ProtoMessage() {}

func (x *GetWechatConfigDetailResponse) ProtoReflect() protoreflect.Message {
	mi := &file_wechat_config_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetWechatConfigDetailResponse.ProtoReflect.Descriptor instead.
func (*GetWechatConfigDetailResponse) Descriptor() ([]byte, []int) {
	return file_wechat_config_proto_rawDescGZIP(), []int{4}
}

func (x *GetWechatConfigDetailResponse) GetCode() Code {
	if x != nil {
		return x.Code
	}
	return Code_None
}

func (x *GetWechatConfigDetailResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *GetWechatConfigDetailResponse) GetData() *WechatConfigInfo {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_wechat_config_proto protoreflect.FileDescriptor

var file_wechat_config_proto_rawDesc = []byte{
	0x0a, 0x13, 0x77, 0x65, 0x63, 0x68, 0x61, 0x74, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0a, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65,
	0x72, 0x1a, 0x0c, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0xa6, 0x03, 0x0a, 0x10, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x49, 0x6e, 0x66, 0x6f, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x02, 0x69, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x70, 0x70, 0x49, 0x44, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x05, 0x61, 0x70, 0x70, 0x49, 0x44, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x70,
	0x70, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x70, 0x70,
	0x4e, 0x61, 0x6d, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x12, 0x1a, 0x0a, 0x08,
	0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08,
	0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x70, 0x70, 0x54,
	0x79, 0x70, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x05, 0x52, 0x07, 0x61, 0x70, 0x70, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x24, 0x0a, 0x0d, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x52, 0x6f, 0x6c,
	0x65, 0x49, 0x44, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x64, 0x65, 0x66, 0x61, 0x75,
	0x6c, 0x74, 0x52, 0x6f, 0x6c, 0x65, 0x49, 0x44, 0x12, 0x20, 0x0a, 0x0b, 0x72, 0x65, 0x64, 0x69,
	0x72, 0x65, 0x63, 0x74, 0x55, 0x72, 0x6c, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x72,
	0x65, 0x64, 0x69, 0x72, 0x65, 0x63, 0x74, 0x55, 0x72, 0x6c, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f,
	0x6b, 0x65, 0x6e, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e,
	0x12, 0x26, 0x0a, 0x0e, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x41, 0x45, 0x53, 0x4b,
	0x65, 0x79, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0e, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69,
	0x6e, 0x67, 0x41, 0x45, 0x53, 0x4b, 0x65, 0x79, 0x12, 0x26, 0x0a, 0x0e, 0x65, 0x6e, 0x63, 0x6f,
	0x64, 0x69, 0x6e, 0x67, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x0e, 0x65, 0x6e, 0x63, 0x6f, 0x64, 0x69, 0x6e, 0x67, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64,
	0x12, 0x20, 0x0a, 0x0b, 0x64, 0x69, 0x73, 0x70, 0x6c, 0x61, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x18,
	0x0c, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x69, 0x73, 0x70, 0x6c, 0x61, 0x79, 0x4e, 0x61,
	0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x44, 0x18,
	0x0d, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x49, 0x44,
	0x12, 0x16, 0x0a, 0x06, 0x69, 0x73, 0x4d, 0x75, 0x73, 0x74, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x06, 0x69, 0x73, 0x4d, 0x75, 0x73, 0x74, 0x22, 0x90, 0x02, 0x0a, 0x18, 0x51, 0x75, 0x65,
	0x72, 0x79, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x70, 0x61, 0x67, 0x65, 0x49, 0x6e, 0x64,
	0x65, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x70, 0x61, 0x67, 0x65, 0x49, 0x6e,
	0x64, 0x65, 0x78, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x61, 0x67, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x70, 0x61, 0x67, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x12,
	0x1e, 0x0a, 0x0a, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0a, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x12,
	0x12, 0x0a, 0x04, 0x64, 0x65, 0x73, 0x63, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x04, 0x64,
	0x65, 0x73, 0x63, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x70, 0x70, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x70, 0x70, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a,
	0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x49, 0x44, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x70, 0x70,
	0x54, 0x79, 0x70, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x05, 0x52, 0x07, 0x61, 0x70, 0x70, 0x54,
	0x79, 0x70, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x69, 0x73, 0x4d, 0x75, 0x73, 0x74, 0x18, 0x0a, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x06, 0x69, 0x73, 0x4d, 0x75, 0x73, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x73,
	0x6f, 0x72, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0a, 0x73, 0x6f, 0x72, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x22, 0xd3, 0x01, 0x0a, 0x19,
	0x51, 0x75, 0x65, 0x72, 0x79, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x24, 0x0a, 0x04, 0x63, 0x6f, 0x64,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x10, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12,
	0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x30, 0x0a, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65,
	0x6e, 0x74, 0x65, 0x72, 0x2e, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x70,
	0x61, 0x67, 0x65, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x70, 0x61, 0x67, 0x65,
	0x73, 0x12, 0x18, 0x0a, 0x07, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x73, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x07, 0x72, 0x65, 0x63, 0x6f, 0x72, 0x64, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x74,
	0x6f, 0x74, 0x61, 0x6c, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x74, 0x6f, 0x74, 0x61,
	0x6c, 0x22, 0x8e, 0x01, 0x0a, 0x1a, 0x47, 0x65, 0x74, 0x41, 0x6c, 0x6c, 0x57, 0x65, 0x63, 0x68,
	0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x24, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x10,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x64, 0x65,
	0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x12, 0x30, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1c,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x57, 0x65, 0x63, 0x68,
	0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x04, 0x64, 0x61,
	0x74, 0x61, 0x22, 0x91, 0x01, 0x0a, 0x1d, 0x47, 0x65, 0x74, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x24, 0x0a, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x10, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x43, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x12, 0x30, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x49, 0x6e, 0x66, 0x6f,
	0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x32, 0xa5, 0x04, 0x0a, 0x0c, 0x57, 0x65, 0x63, 0x68, 0x61,
	0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x41, 0x0a, 0x03, 0x41, 0x64, 0x64, 0x12, 0x1c,
	0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x57, 0x65, 0x63, 0x68,
	0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x49, 0x6e, 0x66, 0x6f, 0x1a, 0x1a, 0x2e, 0x75,
	0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x44, 0x0a, 0x06, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x12, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65,
	0x72, 0x2e, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x49, 0x6e,
	0x66, 0x6f, 0x1a, 0x1a, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x12, 0x3e, 0x0a, 0x06, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x12, 0x16, 0x2e, 0x75, 0x73, 0x65,
	0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x44, 0x65, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x12, 0x56, 0x0a, 0x05, 0x51, 0x75, 0x65, 0x72, 0x79, 0x12, 0x24, 0x2e, 0x75, 0x73, 0x65, 0x72,
	0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x51, 0x75, 0x65, 0x72, 0x79, 0x57, 0x65, 0x63, 0x68,
	0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x25, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x51, 0x75, 0x65,
	0x72, 0x79, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x4d, 0x0a, 0x06, 0x47, 0x65, 0x74, 0x41,
	0x6c, 0x6c, 0x12, 0x19, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x47, 0x65, 0x74, 0x41, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x26, 0x2e,
	0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x47, 0x65, 0x74, 0x41, 0x6c,
	0x6c, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x56, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x44, 0x65,
	0x74, 0x61, 0x69, 0x6c, 0x12, 0x1c, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65,
	0x72, 0x2e, 0x47, 0x65, 0x74, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x29, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e,
	0x47, 0x65, 0x74, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x44,
	0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12,
	0x4d, 0x0a, 0x06, 0x45, 0x78, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x1f, 0x2e, 0x75, 0x73, 0x65, 0x72,
	0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x45, 0x78, 0x70,
	0x6f, 0x72, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x20, 0x2e, 0x75, 0x73, 0x65,
	0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x45, 0x78,
	0x70, 0x6f, 0x72, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x48,
	0x0a, 0x13, 0x63, 0x6e, 0x2e, 0x61, 0x74, 0x61, 0x6c, 0x69, 0x2e, 0x75, 0x73, 0x65, 0x72, 0x63,
	0x65, 0x6e, 0x74, 0x65, 0x72, 0x42, 0x11, 0x57, 0x65, 0x63, 0x68, 0x61, 0x74, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x0d, 0x2e, 0x2f, 0x3b, 0x75,
	0x73, 0x65, 0x72, 0x63, 0x65, 0x6e, 0x74, 0x65, 0x72, 0xa2, 0x02, 0x0c, 0x57, 0x45, 0x42, 0x43,
	0x4f, 0x4e, 0x46, 0x49, 0x47, 0x53, 0x52, 0x56, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_wechat_config_proto_rawDescOnce sync.Once
	file_wechat_config_proto_rawDescData = file_wechat_config_proto_rawDesc
)

func file_wechat_config_proto_rawDescGZIP() []byte {
	file_wechat_config_proto_rawDescOnce.Do(func() {
		file_wechat_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_wechat_config_proto_rawDescData)
	})
	return file_wechat_config_proto_rawDescData
}

var file_wechat_config_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_wechat_config_proto_goTypes = []interface{}{
	(*WechatConfigInfo)(nil),              // 0: usercenter.WechatConfigInfo
	(*QueryWechatConfigRequest)(nil),      // 1: usercenter.QueryWechatConfigRequest
	(*QueryWechatConfigResponse)(nil),     // 2: usercenter.QueryWechatConfigResponse
	(*GetAllWechatConfigResponse)(nil),    // 3: usercenter.GetAllWechatConfigResponse
	(*GetWechatConfigDetailResponse)(nil), // 4: usercenter.GetWechatConfigDetailResponse
	(Code)(0),                             // 5: usercenter.Code
	(*DelRequest)(nil),                    // 6: usercenter.DelRequest
	(*GetAllRequest)(nil),                 // 7: usercenter.GetAllRequest
	(*GetDetailRequest)(nil),              // 8: usercenter.GetDetailRequest
	(*CommonExportRequest)(nil),           // 9: usercenter.CommonExportRequest
	(*CommonResponse)(nil),                // 10: usercenter.CommonResponse
	(*CommonExportResponse)(nil),          // 11: usercenter.CommonExportResponse
}
var file_wechat_config_proto_depIdxs = []int32{
	5,  // 0: usercenter.QueryWechatConfigResponse.code:type_name -> usercenter.Code
	0,  // 1: usercenter.QueryWechatConfigResponse.data:type_name -> usercenter.WechatConfigInfo
	5,  // 2: usercenter.GetAllWechatConfigResponse.code:type_name -> usercenter.Code
	0,  // 3: usercenter.GetAllWechatConfigResponse.data:type_name -> usercenter.WechatConfigInfo
	5,  // 4: usercenter.GetWechatConfigDetailResponse.code:type_name -> usercenter.Code
	0,  // 5: usercenter.GetWechatConfigDetailResponse.data:type_name -> usercenter.WechatConfigInfo
	0,  // 6: usercenter.WechatConfig.Add:input_type -> usercenter.WechatConfigInfo
	0,  // 7: usercenter.WechatConfig.Update:input_type -> usercenter.WechatConfigInfo
	6,  // 8: usercenter.WechatConfig.Delete:input_type -> usercenter.DelRequest
	1,  // 9: usercenter.WechatConfig.Query:input_type -> usercenter.QueryWechatConfigRequest
	7,  // 10: usercenter.WechatConfig.GetAll:input_type -> usercenter.GetAllRequest
	8,  // 11: usercenter.WechatConfig.GetDetail:input_type -> usercenter.GetDetailRequest
	9,  // 12: usercenter.WechatConfig.Export:input_type -> usercenter.CommonExportRequest
	10, // 13: usercenter.WechatConfig.Add:output_type -> usercenter.CommonResponse
	10, // 14: usercenter.WechatConfig.Update:output_type -> usercenter.CommonResponse
	10, // 15: usercenter.WechatConfig.Delete:output_type -> usercenter.CommonResponse
	2,  // 16: usercenter.WechatConfig.Query:output_type -> usercenter.QueryWechatConfigResponse
	3,  // 17: usercenter.WechatConfig.GetAll:output_type -> usercenter.GetAllWechatConfigResponse
	4,  // 18: usercenter.WechatConfig.GetDetail:output_type -> usercenter.GetWechatConfigDetailResponse
	11, // 19: usercenter.WechatConfig.Export:output_type -> usercenter.CommonExportResponse
	13, // [13:20] is the sub-list for method output_type
	6,  // [6:13] is the sub-list for method input_type
	6,  // [6:6] is the sub-list for extension type_name
	6,  // [6:6] is the sub-list for extension extendee
	0,  // [0:6] is the sub-list for field type_name
}

func init() { file_wechat_config_proto_init() }
func file_wechat_config_proto_init() {
	if File_wechat_config_proto != nil {
		return
	}
	file_common_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_wechat_config_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WechatConfigInfo); i {
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
		file_wechat_config_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueryWechatConfigRequest); i {
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
		file_wechat_config_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*QueryWechatConfigResponse); i {
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
		file_wechat_config_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetAllWechatConfigResponse); i {
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
		file_wechat_config_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetWechatConfigDetailResponse); i {
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
			RawDescriptor: file_wechat_config_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_wechat_config_proto_goTypes,
		DependencyIndexes: file_wechat_config_proto_depIdxs,
		MessageInfos:      file_wechat_config_proto_msgTypes,
	}.Build()
	File_wechat_config_proto = out.File
	file_wechat_config_proto_rawDesc = nil
	file_wechat_config_proto_goTypes = nil
	file_wechat_config_proto_depIdxs = nil
}
