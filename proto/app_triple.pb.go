﻿// Code generated by protoc-gen-go-triple. DO NOT EDIT.
// versions:
// - protoc-gen-go-triple v1.0.8
// - protoc             v3.20.3
// source: app.proto

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

// APPClient is the client API for APP service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type APPClient interface {
	Export(ctx context.Context, in *CommonExportRequest, opts ...grpc_go.CallOption) (*CommonExportResponse, common.ErrorWithAttachment)
}

type aPPClient struct {
	cc *triple.TripleConn
}

type APPClientImpl struct {
	Export func(ctx context.Context, in *CommonExportRequest) (*CommonExportResponse, error)
}

func (c *APPClientImpl) GetDubboStub(cc *triple.TripleConn) APPClient {
	return NewAPPClient(cc)
}

func (c *APPClientImpl) XXX_InterfaceName() string {
	return "usercenter.APP"
}

func NewAPPClient(cc *triple.TripleConn) APPClient {
	return &aPPClient{cc}
}

func (c *aPPClient) Export(ctx context.Context, in *CommonExportRequest, opts ...grpc_go.CallOption) (*CommonExportResponse, common.ErrorWithAttachment) {
	out := new(CommonExportResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Export", in, out)
}

// APPServer is the server API for APP service.
// All implementations must embed UnimplementedAPPServer
// for forward compatibility
type APPServer interface {
	Export(context.Context, *CommonExportRequest) (*CommonExportResponse, error)
	mustEmbedUnimplementedAPPServer()
}

// UnimplementedAPPServer must be embedded to have forward compatible implementations.
type UnimplementedAPPServer struct {
	proxyImpl protocol.Invoker
}

func (UnimplementedAPPServer) Export(context.Context, *CommonExportRequest) (*CommonExportResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Export not implemented")
}
func (s *UnimplementedAPPServer) XXX_SetProxyImpl(impl protocol.Invoker) {
	s.proxyImpl = impl
}

func (s *UnimplementedAPPServer) XXX_GetProxyImpl() protocol.Invoker {
	return s.proxyImpl
}

func (s *UnimplementedAPPServer) XXX_ServiceDesc() *grpc_go.ServiceDesc {
	return &APP_ServiceDesc
}
func (s *UnimplementedAPPServer) XXX_InterfaceName() string {
	return "usercenter.APP"
}

func (UnimplementedAPPServer) mustEmbedUnimplementedAPPServer() {}

// UnsafeAPPServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to APPServer will
// result in compilation errors.
type UnsafeAPPServer interface {
	mustEmbedUnimplementedAPPServer()
}

func RegisterAPPServer(s grpc_go.ServiceRegistrar, srv APPServer) {
	s.RegisterService(&APP_ServiceDesc, srv)
}

func _APP_Export_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
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

// APP_ServiceDesc is the grpc_go.ServiceDesc for APP service.
// It's only intended for direct use with grpc_go.RegisterService,
// and not to be introspected or modified (even as a copy)
var APP_ServiceDesc = grpc_go.ServiceDesc{
	ServiceName: "usercenter.APP",
	HandlerType: (*APPServer)(nil),
	Methods: []grpc_go.MethodDesc{
		{
			MethodName: "Export",
			Handler:    _APP_Export_Handler,
		},
	},
	Streams:  []grpc_go.StreamDesc{},
	Metadata: "app.proto",
}
