﻿// Code generated by protoc-gen-go-triple. DO NOT EDIT.
// versions:
// - protoc-gen-go-triple v1.0.8
// - protoc             v3.20.3
// source: form_component.proto

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

// FormComponentClient is the client API for FormComponent service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type FormComponentClient interface {
	Add(ctx context.Context, in *FormComponentInfo, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment)
	Update(ctx context.Context, in *FormComponentInfo, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment)
	Delete(ctx context.Context, in *DelRequest, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment)
	Query(ctx context.Context, in *QueryFormComponentRequest, opts ...grpc_go.CallOption) (*QueryFormComponentResponse, common.ErrorWithAttachment)
	GetDetail(ctx context.Context, in *GetDetailRequest, opts ...grpc_go.CallOption) (*GetFormComponentDetailResponse, common.ErrorWithAttachment)
}

type formComponentClient struct {
	cc *triple.TripleConn
}

type FormComponentClientImpl struct {
	Add       func(ctx context.Context, in *FormComponentInfo) (*CommonResponse, error)
	Update    func(ctx context.Context, in *FormComponentInfo) (*CommonResponse, error)
	Delete    func(ctx context.Context, in *DelRequest) (*CommonResponse, error)
	Query     func(ctx context.Context, in *QueryFormComponentRequest) (*QueryFormComponentResponse, error)
	GetDetail func(ctx context.Context, in *GetDetailRequest) (*GetFormComponentDetailResponse, error)
}

func (c *FormComponentClientImpl) GetDubboStub(cc *triple.TripleConn) FormComponentClient {
	return NewFormComponentClient(cc)
}

func (c *FormComponentClientImpl) XXX_InterfaceName() string {
	return "usercenter.FormComponent"
}

func NewFormComponentClient(cc *triple.TripleConn) FormComponentClient {
	return &formComponentClient{cc}
}

func (c *formComponentClient) Add(ctx context.Context, in *FormComponentInfo, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment) {
	out := new(CommonResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Add", in, out)
}

func (c *formComponentClient) Update(ctx context.Context, in *FormComponentInfo, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment) {
	out := new(CommonResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Update", in, out)
}

func (c *formComponentClient) Delete(ctx context.Context, in *DelRequest, opts ...grpc_go.CallOption) (*CommonResponse, common.ErrorWithAttachment) {
	out := new(CommonResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Delete", in, out)
}

func (c *formComponentClient) Query(ctx context.Context, in *QueryFormComponentRequest, opts ...grpc_go.CallOption) (*QueryFormComponentResponse, common.ErrorWithAttachment) {
	out := new(QueryFormComponentResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Query", in, out)
}

func (c *formComponentClient) GetDetail(ctx context.Context, in *GetDetailRequest, opts ...grpc_go.CallOption) (*GetFormComponentDetailResponse, common.ErrorWithAttachment) {
	out := new(GetFormComponentDetailResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/GetDetail", in, out)
}

// FormComponentServer is the server API for FormComponent service.
// All implementations must embed UnimplementedFormComponentServer
// for forward compatibility
type FormComponentServer interface {
	Add(context.Context, *FormComponentInfo) (*CommonResponse, error)
	Update(context.Context, *FormComponentInfo) (*CommonResponse, error)
	Delete(context.Context, *DelRequest) (*CommonResponse, error)
	Query(context.Context, *QueryFormComponentRequest) (*QueryFormComponentResponse, error)
	GetDetail(context.Context, *GetDetailRequest) (*GetFormComponentDetailResponse, error)
	mustEmbedUnimplementedFormComponentServer()
}

// UnimplementedFormComponentServer must be embedded to have forward compatible implementations.
type UnimplementedFormComponentServer struct {
	proxyImpl protocol.Invoker
}

func (UnimplementedFormComponentServer) Add(context.Context, *FormComponentInfo) (*CommonResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Add not implemented")
}
func (UnimplementedFormComponentServer) Update(context.Context, *FormComponentInfo) (*CommonResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedFormComponentServer) Delete(context.Context, *DelRequest) (*CommonResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedFormComponentServer) Query(context.Context, *QueryFormComponentRequest) (*QueryFormComponentResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Query not implemented")
}
func (UnimplementedFormComponentServer) GetDetail(context.Context, *GetDetailRequest) (*GetFormComponentDetailResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDetail not implemented")
}
func (s *UnimplementedFormComponentServer) XXX_SetProxyImpl(impl protocol.Invoker) {
	s.proxyImpl = impl
}

func (s *UnimplementedFormComponentServer) XXX_GetProxyImpl() protocol.Invoker {
	return s.proxyImpl
}

func (s *UnimplementedFormComponentServer) XXX_ServiceDesc() *grpc_go.ServiceDesc {
	return &FormComponent_ServiceDesc
}
func (s *UnimplementedFormComponentServer) XXX_InterfaceName() string {
	return "usercenter.FormComponent"
}

func (UnimplementedFormComponentServer) mustEmbedUnimplementedFormComponentServer() {}

// UnsafeFormComponentServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to FormComponentServer will
// result in compilation errors.
type UnsafeFormComponentServer interface {
	mustEmbedUnimplementedFormComponentServer()
}

func RegisterFormComponentServer(s grpc_go.ServiceRegistrar, srv FormComponentServer) {
	s.RegisterService(&FormComponent_ServiceDesc, srv)
}

func _FormComponent_Add_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(FormComponentInfo)
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

func _FormComponent_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(FormComponentInfo)
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

func _FormComponent_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
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

func _FormComponent_Query_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryFormComponentRequest)
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

func _FormComponent_GetDetail_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
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

// FormComponent_ServiceDesc is the grpc_go.ServiceDesc for FormComponent service.
// It's only intended for direct use with grpc_go.RegisterService,
// and not to be introspected or modified (even as a copy)
var FormComponent_ServiceDesc = grpc_go.ServiceDesc{
	ServiceName: "usercenter.FormComponent",
	HandlerType: (*FormComponentServer)(nil),
	Methods: []grpc_go.MethodDesc{
		{
			MethodName: "Add",
			Handler:    _FormComponent_Add_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _FormComponent_Update_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _FormComponent_Delete_Handler,
		},
		{
			MethodName: "Query",
			Handler:    _FormComponent_Query_Handler,
		},
		{
			MethodName: "GetDetail",
			Handler:    _FormComponent_GetDetail_Handler,
		},
	},
	Streams:  []grpc_go.StreamDesc{},
	Metadata: "form_component.proto",
}
