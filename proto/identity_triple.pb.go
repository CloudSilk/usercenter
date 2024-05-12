// Code generated by protoc-gen-go-triple. DO NOT EDIT.
// versions:
// - protoc-gen-go-triple v1.0.8
// - protoc             v3.20.3
// source: identity.proto

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

// IdentityClient is the client API for Identity service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type IdentityClient interface {
	Authenticate(ctx context.Context, in *AuthenticateRequest, opts ...grpc_go.CallOption) (*AuthenticateResponse, common.ErrorWithAttachment)
	DecodeToken(ctx context.Context, in *DecodeTokenRequest, opts ...grpc_go.CallOption) (*AuthenticateResponse, common.ErrorWithAttachment)
}

type identityClient struct {
	cc *triple.TripleConn
}

type IdentityClientImpl struct {
	Authenticate func(ctx context.Context, in *AuthenticateRequest) (*AuthenticateResponse, error)
	DecodeToken  func(ctx context.Context, in *DecodeTokenRequest) (*AuthenticateResponse, error)
}

func (c *IdentityClientImpl) GetDubboStub(cc *triple.TripleConn) IdentityClient {
	return NewIdentityClient(cc)
}

func (c *IdentityClientImpl) XXX_InterfaceName() string {
	return "usercenter.Identity"
}

func NewIdentityClient(cc *triple.TripleConn) IdentityClient {
	return &identityClient{cc}
}

func (c *identityClient) Authenticate(ctx context.Context, in *AuthenticateRequest, opts ...grpc_go.CallOption) (*AuthenticateResponse, common.ErrorWithAttachment) {
	out := new(AuthenticateResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/Authenticate", in, out)
}

func (c *identityClient) DecodeToken(ctx context.Context, in *DecodeTokenRequest, opts ...grpc_go.CallOption) (*AuthenticateResponse, common.ErrorWithAttachment) {
	out := new(AuthenticateResponse)
	interfaceKey := ctx.Value(constant.InterfaceKey).(string)
	return out, c.cc.Invoke(ctx, "/"+interfaceKey+"/DecodeToken", in, out)
}

// IdentityServer is the server API for Identity service.
// All implementations must embed UnimplementedIdentityServer
// for forward compatibility
type IdentityServer interface {
	Authenticate(context.Context, *AuthenticateRequest) (*AuthenticateResponse, error)
	DecodeToken(context.Context, *DecodeTokenRequest) (*AuthenticateResponse, error)
	mustEmbedUnimplementedIdentityServer()
}

// UnimplementedIdentityServer must be embedded to have forward compatible implementations.
type UnimplementedIdentityServer struct {
	proxyImpl protocol.Invoker
}

func (UnimplementedIdentityServer) Authenticate(context.Context, *AuthenticateRequest) (*AuthenticateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Authenticate not implemented")
}
func (UnimplementedIdentityServer) DecodeToken(context.Context, *DecodeTokenRequest) (*AuthenticateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DecodeToken not implemented")
}
func (s *UnimplementedIdentityServer) XXX_SetProxyImpl(impl protocol.Invoker) {
	s.proxyImpl = impl
}

func (s *UnimplementedIdentityServer) XXX_GetProxyImpl() protocol.Invoker {
	return s.proxyImpl
}

func (s *UnimplementedIdentityServer) XXX_ServiceDesc() *grpc_go.ServiceDesc {
	return &Identity_ServiceDesc
}
func (s *UnimplementedIdentityServer) XXX_InterfaceName() string {
	return "usercenter.Identity"
}

func (UnimplementedIdentityServer) mustEmbedUnimplementedIdentityServer() {}

// UnsafeIdentityServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to IdentityServer will
// result in compilation errors.
type UnsafeIdentityServer interface {
	mustEmbedUnimplementedIdentityServer()
}

func RegisterIdentityServer(s grpc_go.ServiceRegistrar, srv IdentityServer) {
	s.RegisterService(&Identity_ServiceDesc, srv)
}

func _Identity_Authenticate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(AuthenticateRequest)
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
	invo := invocation.NewRPCInvocation("Authenticate", args, invAttachment)
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

func _Identity_DecodeToken_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc_go.UnaryServerInterceptor) (interface{}, error) {
	in := new(DecodeTokenRequest)
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
	invo := invocation.NewRPCInvocation("DecodeToken", args, invAttachment)
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

// Identity_ServiceDesc is the grpc_go.ServiceDesc for Identity service.
// It's only intended for direct use with grpc_go.RegisterService,
// and not to be introspected or modified (even as a copy)
var Identity_ServiceDesc = grpc_go.ServiceDesc{
	ServiceName: "usercenter.Identity",
	HandlerType: (*IdentityServer)(nil),
	Methods: []grpc_go.MethodDesc{
		{
			MethodName: "Authenticate",
			Handler:    _Identity_Authenticate_Handler,
		},
		{
			MethodName: "DecodeToken",
			Handler:    _Identity_DecodeToken_Handler,
		},
	},
	Streams:  []grpc_go.StreamDesc{},
	Metadata: "identity.proto",
}
