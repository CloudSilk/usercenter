package provider

import (
	"context"

	"github.com/CloudSilk/usercenter/internal/app"
	apipb "github.com/CloudSilk/usercenter/proto"
)

type APPProvider struct {
	apipb.UnimplementedAPPServer
}

func (u *APPProvider) Export(ctx context.Context, in *apipb.CommonExportRequest) (*apipb.CommonExportResponse, error) {
	resp := &apipb.CommonExportResponse{
		Code: apipb.Code_Success,
	}

	app.ExportAllAPPs(in, resp)

	return resp, nil
}
