package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/internal/authn"
	"github.com/CloudSilk/usercenter/internal/auth/token"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type IdentityProvider struct {
	apipb.UnimplementedIdentityServer
}

func (u *IdentityProvider) Authenticate(ctx context.Context, in *apipb.AuthenticateRequest) (*apipb.AuthenticateResponse, error) {
	resp := &apipb.AuthenticateResponse{
		Code: apipb.Code_Success,
	}
	_, currentUser, code, err := authn.AuthenticatePrincipal(in.Token, in.Method, in.Url, in.CheckAuth)
	if code != int(apipb.Code_Success) {
		resp.Code = apipb.Code(code)
		if err != nil {
			resp.Message = err.Error()
		}
	} else {
		resp.CurrentUser = currentUser
		resp.Code = apipb.Code(code)
	}
	return resp, nil
}

func (u *IdentityProvider) DecodeToken(ctx context.Context, in *apipb.DecodeTokenRequest) (*apipb.AuthenticateResponse, error) {
	resp := &apipb.AuthenticateResponse{
		Code: apipb.Code_Success,
	}
	currentUser, err := token.DecodeToken(in.Token)
	if err != nil {
		resp.Code = apipb.Code_InternalServerError
		resp.Message = err.Error()
	} else {
		resp.CurrentUser = currentUser
		resp.Code = apipb.Code_Success
	}
	return resp, nil
}
