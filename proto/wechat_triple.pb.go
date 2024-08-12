﻿// Code generated by protoc-gen-go-triple. DO NOT EDIT.
// versions:
// - protoc-gen-go-triple v1.0.8
// - protoc             v3.20.3
// source: wechat.proto

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

// WechatClient is the client API for Wechat service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type WechatClient interface {
	SendTplMsg(ctx context.Context, in *SendTplMsgRequest, opts ...grpc_go.CallOption) (*SendTplMsgResponse, common.ErrorWithAttachment)
}

type wechatClient struct {
	cc *triple.TripleConn
}

type WechatClientImpl struct {
	SendTplMsg func(ctx context.Context, in *SendTplMsgRequest) (*SendTplMsgResponse, error)
}

func (c *WechatClientImpl) GetDubboStub(cc *triple.TripleConn) WechatClient {
	return NewWechatClient(cc)
}

func (c *WechatClientImpl) XXX_InterfaceName() string {
	return "usercenter.Wechat"
}

func NewWechatClient(cc *triple.TripleConn) WechatClient {
	return &wechatClient{cc}
}

func (c *wechatClient) SendTplMsg(ctx context.Context, in *SendTplMsgRequest, opts ...grpc_go.CallOption) (*SendTplMsgResponse, common.ErrorWithAttachment) {
	out := new(SendTplMsgResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/SendTplMsg", in, out)
}

// WechatServer is the server API for Wechat service.
// All implementations must embed UnimplementedWechatServer
// for forward compatibility
type WechatServer interface {
	SendTplMsg(context.Context, *SendTplMsgRequest) (*SendTplMsgResponse, error)
	mustEmbedUnimplementedWechatServer()
}

// UnimplementedWechatServer must be embedded to have forward compatible implementations.
type UnimplementedWechatServer struct {
	proxyImpl protocol.Invoker
}

func (UnimplementedWechatServer) SendTplMsg(context.Context, *SendTplMsgRequest) (*SendTplMsgResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendTplMsg not implemented")
}
func (s *UnimplementedWechatServer) XXX_SetProxyImpl(impl protocol.Invoker) {
	s.proxyImpl = impl
}

func (s *UnimplementedWechatServer) XXX_GetProxyImpl() protocol.Invoker {
	return s.proxyImpl
}

func (s *UnimplementedWechatServer) XXX_ServiceDesc() *grpc_go.ServiceDesc {
	return &Wechat_ServiceDesc
}
func (s *UnimplementedWechatServer) XXX_InterfaceName() string {
	return "usercenter.Wechat"
}

func (UnimplementedWechatServer) mustEmbedUnimplementedWechatServer() {}

// UnsafeWechatServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to WechatServer will
// result in compilation errors.
type UnsafeWechatServer interface {
	mustEmbedUnimplementedWechatServer()
}

func RegisterWechatServer(s grpc_go.ServiceRegistrar, srv WechatServer) {
	s.RegisterService(&Wechat_ServiceDesc, srv)
}

func _Wechat_SendTplMsg_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendTplMsgRequest)
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
	invo := invocation.NewRPCInvocation("SendTplMsg", args, invAttachment)
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

// Wechat_ServiceDesc is the grpc_go.ServiceDesc for Wechat service.
// It's only intended for direct use with grpc_go.RegisterService,
// and not to be introspected or modified (even as a copy)
var Wechat_ServiceDesc = grpc_go.ServiceDesc{
	ServiceName: "usercenter.Wechat",
	HandlerType: (*WechatServer)(nil),
	Methods: []grpc_go.MethodDesc{
		{
			MethodName: "SendTplMsg",
			Handler:    _Wechat_SendTplMsg_Handler,
		},
	},
	Streams:  []grpc_go.StreamDesc{},
	Metadata: "wechat.proto",
}
