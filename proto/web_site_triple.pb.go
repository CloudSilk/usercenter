// Code generated by protoc-gen-go-triple. DO NOT EDIT.
// versions:
// - protoc-gen-go-triple v1.0.8
// - protoc             v3.20.3
// source: web_site.proto

package usercenter

import (
	context "context"
	protocol "dubbo.apache.org/dubbo-go/v3/protocol"
	dubbo3 "dubbo.apache.org/dubbo-go/v3/protocol/dubbo3"
	invocation "dubbo.apache.org/dubbo-go/v3/protocol/invocation"
	grpc_go "github.com/dubbogo/grpc-go"
	codes "github.com/dubbogo/grpc-go/codes"
	metadata "github.com/dubbogo/grpc-go/metadata"
	status "github.com/dubbogo/grpc-go/status"
	common "github.com/dubbogo/triple/pkg/common"
	constant "github.com/dubbogo/triple/pkg/common/constant"
	triple "github.com/dubbogo/triple/pkg/triple"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc_go.SupportPackageIsVersion7

// WebSiteClient is the client API for WebSite service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type WebSiteClient interface {
	Add(ctx context.Context, in *WebSiteInfo, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment)
	Update(ctx context.Context, in *WebSiteInfo, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment)
	Delete(ctx context.Context, in *DelRequest, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment)
	Query(ctx context.Context, in *QueryWebSiteRequest, opts ...grpc_go.CallOption) (*QueryWebSiteResponse, common.ErrorWithAttachment)
	GetAll(ctx context.Context, in *GetAllRequest, opts ...grpc_go.CallOption) (*GetAllWebSiteResponse, common.ErrorWithAttachment)
	GetDetail(ctx context.Context, in *GetDetailRequest, opts ...grpc_go.CallOption) (*GetWebSiteDetailResponse, common.ErrorWithAttachment)
	Export(ctx context.Context, in *CommonExportRequest, opts ...grpc_go.CallOption) (*CommonExportResponse, common.ErrorWithAttachment)
}

type webSiteClient struct {
	cc *triple.TripleConn
}

type WebSiteClientImpl struct {
	Add       func(ctx context.Context, in *WebSiteInfo) (*CommonResponse, error)
	Update    func(ctx context.Context, in *WebSiteInfo) (*CommonResponse, error)
	Delete    func(ctx context.Context, in *DelRequest) (*CommonResponse, error)
	Query     func(ctx context.Context, in *QueryWebSiteRequest) (*QueryWebSiteResponse, error)
	GetAll    func(ctx context.Context, in *GetAllRequest) (*GetAllWebSiteResponse, error)
	GetDetail func(ctx context.Context, in *GetDetailRequest) (*GetWebSiteDetailResponse, error)
	Export    func(ctx context.Context, in *CommonExportRequest) (*CommonExportResponse, error)
}

func (c *WebSiteClientImpl) GetDubboStub(cc *triple.TripleConn) WebSiteClient {
	return NewWebSiteClient(cc)
}

func (c *WebSiteClientImpl) XXX_InterfaceName() string {
	return "usercenter.WebSite"
}

func NewWebSiteClient(cc *triple.TripleConn) WebSiteClient {
	return &webSiteClient{cc}
}

func (c *webSiteClient) Add(ctx context.Context, in *WebSiteInfo, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment) {
	out := new(CommonResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Add", in, out)
}

func (c *webSiteClient) Update(ctx context.Context, in *WebSiteInfo, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment) {
	out := new(CommonResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Update", in, out)
}

func (c *webSiteClient) Delete(ctx context.Context, in *DelRequest, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment) {
	out := new(CommonResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Delete", in, out)
}

func (c *webSiteClient) Query(ctx context.Context, in *QueryWebSiteRequest, opts ...grpc_go.CallOption) (*QueryWebSiteResponse, common.ErrorWithAttachment) {
	out := new(QueryWebSiteResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Query", in, out)
}

func (c *webSiteClient) GetAll(ctx context.Context, in *GetAllRequest, opts ...grpc_go.CallOption) (*GetAllWebSiteResponse, common.ErrorWithAttachment) {
	out := new(GetAllWebSiteResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/GetAll", in, out)
}

func (c *webSiteClient) GetDetail(ctx context.Context, in *GetDetailRequest, opts ...grpc_go.CallOption) (*GetWebSiteDetailResponse, common.ErrorWithAttachment) {
	out := new(GetWebSiteDetailResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/GetDetail", in, out)
}

func (c *webSiteClient) Export(ctx context.Context, in *CommonExportRequest, opts ...grpc_go.CallOption) (*CommonExportResponse, common.ErrorWithAttachment) {
	out := new(CommonExportResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Export", in, out)
}

// WebSiteServer is the server API for WebSite service.
// All implementations must embed UnimplementedWebSiteServer
// for forward compatibility
type WebSiteServer interface {
	Add(context.Context, *WebSiteInfo) (*CommonResponse, error)
	Update(context.Context, *WebSiteInfo) (*CommonResponse, error)
	Delete(context.Context, *DelRequest) (*CommonResponse, error)
	Query(context.Context, *QueryWebSiteRequest) (*QueryWebSiteResponse, error)
	GetAll(context.Context, *GetAllRequest) (*GetAllWebSiteResponse, error)
	GetDetail(context.Context, *GetDetailRequest) (*GetWebSiteDetailResponse, error)
	Export(context.Context, *CommonExportRequest) (*CommonExportResponse, error)
	mustEmbedUnimplementedWebSiteServer()
}

// UnimplementedWebSiteServer must be embedded to have forward compatible implementations.
type UnimplementedWebSiteServer struct {
	proxyImpl protocol.Invoker
}

func (UnimplementedWebSiteServer) Add(context.Context, *WebSiteInfo) (*CommonResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Add not implemented")
}
func (UnimplementedWebSiteServer) Update(context.Context, *WebSiteInfo) (*CommonResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedWebSiteServer) Delete(context.Context, *DelRequest) (*CommonResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedWebSiteServer) Query(context.Context, *QueryWebSiteRequest) (*QueryWebSiteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Query not implemented")
}
func (UnimplementedWebSiteServer) GetAll(context.Context, *GetAllRequest) (*GetAllWebSiteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAll not implemented")
}
func (UnimplementedWebSiteServer) GetDetail(context.Context, *GetDetailRequest) (*GetWebSiteDetailResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDetail not implemented")
}
func (UnimplementedWebSiteServer) Export(context.Context, *CommonExportRequest) (*CommonExportResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Export not implemented")
}
func (s *UnimplementedWebSiteServer) XXX_SetProxyImpl(impl protocol.Invoker) {
	s.proxyImpl = impl
}

func (s *UnimplementedWebSiteServer) XXX_GetProxyImpl() protocol.Invoker {
	return s.proxyImpl
}

func (s *UnimplementedWebSiteServer) XXX_ServiceDesc() *grpc_go.ServiceDesc {
	return &WebSite_ServiceDesc
}
func (s *UnimplementedWebSiteServer) XXX_InterfaceName() string {
	return "usercenter.WebSite"
}

func (UnimplementedWebSiteServer) mustEmbedUnimplementedWebSiteServer() {}

// UnsafeWebSiteServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to WebSiteServer will
// result in compilation errors.
type UnsafeWebSiteServer interface {
	mustEmbedUnimplementedWebSiteServer()
}

func RegisterWebSiteServer(s grpc_go.ServiceRegistrar, srv WebSiteServer) {
	s.RegisterService(&WebSite_ServiceDesc, srv)
}

func _WebSite_Add_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(WebSiteInfo)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	md, _ := metadata.FromIncomingContext(ctx)
	invAttachment := make(map[string]interface{}, len(md))
	for k, v := range md {
		invAttachment[k] = v
	}
	invo := invocation.NewRPCInvocation("Add", args, invAttachment)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc_go.UnaryServerInfo{
		Server:     srv,
		FullMethod: ctx.Value("XXX_TRIPLE_GO_INTERFACE_NAME").(string),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	return interceptor(ctx, in, info, handler)
}

func _WebSite_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(WebSiteInfo)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	md, _ := metadata.FromIncomingContext(ctx)
	invAttachment := make(map[string]interface{}, len(md))
	for k, v := range md {
		invAttachment[k] = v
	}
	invo := invocation.NewRPCInvocation("Update", args, invAttachment)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc_go.UnaryServerInfo{
		Server:     srv,
		FullMethod: ctx.Value("XXX_TRIPLE_GO_INTERFACE_NAME").(string),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	return interceptor(ctx, in, info, handler)
}

func _WebSite_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(DelRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	md, _ := metadata.FromIncomingContext(ctx)
	invAttachment := make(map[string]interface{}, len(md))
	for k, v := range md {
		invAttachment[k] = v
	}
	invo := invocation.NewRPCInvocation("Delete", args, invAttachment)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc_go.UnaryServerInfo{
		Server:     srv,
		FullMethod: ctx.Value("XXX_TRIPLE_GO_INTERFACE_NAME").(string),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	return interceptor(ctx, in, info, handler)
}

func _WebSite_Query_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryWebSiteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	md, _ := metadata.FromIncomingContext(ctx)
	invAttachment := make(map[string]interface{}, len(md))
	for k, v := range md {
		invAttachment[k] = v
	}
	invo := invocation.NewRPCInvocation("Query", args, invAttachment)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc_go.UnaryServerInfo{
		Server:     srv,
		FullMethod: ctx.Value("XXX_TRIPLE_GO_INTERFACE_NAME").(string),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	return interceptor(ctx, in, info, handler)
}

func _WebSite_GetAll_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAllRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	md, _ := metadata.FromIncomingContext(ctx)
	invAttachment := make(map[string]interface{}, len(md))
	for k, v := range md {
		invAttachment[k] = v
	}
	invo := invocation.NewRPCInvocation("GetAll", args, invAttachment)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc_go.UnaryServerInfo{
		Server:     srv,
		FullMethod: ctx.Value("XXX_TRIPLE_GO_INTERFACE_NAME").(string),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	return interceptor(ctx, in, info, handler)
}

func _WebSite_GetDetail_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetDetailRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	md, _ := metadata.FromIncomingContext(ctx)
	invAttachment := make(map[string]interface{}, len(md))
	for k, v := range md {
		invAttachment[k] = v
	}
	invo := invocation.NewRPCInvocation("GetDetail", args, invAttachment)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc_go.UnaryServerInfo{
		Server:     srv,
		FullMethod: ctx.Value("XXX_TRIPLE_GO_INTERFACE_NAME").(string),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	return interceptor(ctx, in, info, handler)
}

func _WebSite_Export_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(CommonExportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	base := srv.(dubbo3.Dubbo3GrpcService)
	args := []interface{}{}
	args = append(args, in)
	md, _ := metadata.FromIncomingContext(ctx)
	invAttachment := make(map[string]interface{}, len(md))
	for k, v := range md {
		invAttachment[k] = v
	}
	invo := invocation.NewRPCInvocation("Export", args, invAttachment)
	if interceptor == nil {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	info := &grpc_go.UnaryServerInfo{
		Server:     srv,
		FullMethod: ctx.Value("XXX_TRIPLE_GO_INTERFACE_NAME").(string),
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		result := base.XXX_GetProxyImpl().Invoke(ctx, invo)
		return result, result.Error()
	}
	return interceptor(ctx, in, info, handler)
}

// WebSite_ServiceDesc is the grpc_go.ServiceDesc for WebSite service.
// It's only intended for direct use with grpc_go.RegisterService,
// and not to be introspected or modified (even as a copy)
var WebSite_ServiceDesc = grpc_go.ServiceDesc{
	ServiceName: "usercenter.WebSite",
	HandlerType: (*WebSiteServer)(nil),
	Methods: []grpc_go.MethodDesc{
		{
			MethodName: "Add",
			Handler:    _WebSite_Add_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _WebSite_Update_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _WebSite_Delete_Handler,
		},
		{
			MethodName: "Query",
			Handler:    _WebSite_Query_Handler,
		},
		{
			MethodName: "GetAll",
			Handler:    _WebSite_GetAll_Handler,
		},
		{
			MethodName: "GetDetail",
			Handler:    _WebSite_GetDetail_Handler,
		},
		{
			MethodName: "Export",
			Handler:    _WebSite_Export_Handler,
		},
	},
	Streams:  []grpc_go.StreamDesc{},
	Metadata: "web_site.proto",
}
