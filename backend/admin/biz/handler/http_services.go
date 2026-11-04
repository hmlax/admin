package handler

import (
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/tx7do/go-utils/crypto"

	apiV1 "go-admin/admin/biz/model/admin/v1"
	"go-admin/admin/biz/mw"
	"go-admin/admin/internal/transport"
	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/httpmeta"
	"go-admin/platform/pkg/middleware/auth"
	"go-admin/platform/pkg/portalscope"
)

type ExportHTTPService struct {
	client operatorV1.ExportServiceClient
}

func (s *ExportHTTPService) ExportAuditLogs(ctx context.Context, in *apiV1.AuditExportRequest) (*apiV1.BinaryFileResponse, error) {
	if err := checkExportActor(ctx, true); err != nil {
		return nil, err
	}
	request := toRPCAuditExportRequest(in)
	if err := transport.ValidateRequest(request); err != nil {
		return nil, err
	}
	reply, err := s.client.ExportAuditLogs(ctx, request)
	if err != nil {
		return nil, err
	}
	return toAPIBinaryFileResponse(reply), nil
}
func (s *ExportHTTPService) ExportAIUsage(ctx context.Context, in *apiV1.ExportFileRequest) (*apiV1.BinaryFileResponse, error) {
	if err := checkExportActor(ctx, false); err != nil {
		return nil, err
	}
	request := toRPCExportFileRequest(in)
	if err := transport.ValidateRequest(request); err != nil {
		return nil, err
	}
	reply, err := s.client.ExportAIUsage(ctx, request)
	if err != nil {
		return nil, err
	}
	return toAPIBinaryFileResponse(reply), nil
}
func (s *ExportHTTPService) ExportNotificationDeliveries(ctx context.Context, in *apiV1.ExportFileRequest) (*apiV1.BinaryFileResponse, error) {
	if err := checkExportActor(ctx, true); err != nil {
		return nil, err
	}
	request := toRPCExportFileRequest(in)
	if err := transport.ValidateRequest(request); err != nil {
		return nil, err
	}
	reply, err := s.client.ExportNotificationDeliveries(ctx, request)
	if err != nil {
		return nil, err
	}
	return toAPIBinaryFileResponse(reply), nil
}
func checkExportActor(ctx context.Context, platformOnly bool) error {
	actor, err := auth.FromContext(ctx)
	if err != nil {
		return err
	}
	if platformOnly && !actor.GetIsPlatformAdmin() {
		return operatorV1.ErrorAdminForbidden(mw.ErrorPlatformRequired)
	}
	return nil
}

type MediaHTTPService struct {
	client operatorV1.MediaServiceClient
}

func (s *MediaHTTPService) ReadImage(ctx context.Context, in *apiV1.ReadMediaRequest) (*apiV1.BinaryFileResponse, error) {
	if time.Now().Unix() > in.GetExpires() || !crypto.VerifyData(in.GetPath()+"|"+strconv.FormatInt(in.GetExpires(), 10), in.GetSig()) {
		return nil, operatorV1.ErrorAdminForbidden("invalid or expired media signature")
	}
	request := toRPCReadMediaRequest(in)
	if err := transport.ValidateRequest(request); err != nil {
		return nil, err
	}
	reply, err := s.client.ReadImage(ctx, request)
	if err != nil {
		return nil, err
	}
	return toAPIBinaryFileResponse(reply), nil
}

type EventHTTPService struct {
	client  operatorV1.EventServiceClient
	checker auth.AccessTokenChecker
	gateway operatorV1.GatewayServiceClient
}

func (s *EventHTTPService) Subscribe(ctx context.Context, in *apiV1.EventSubscriptionRequest, send func(*apiV1.EventFrame) error) error {
	token := in.GetToken()
	if tr, ok := httpmeta.FromContext(ctx); ok {
		if value := tr.RequestHeader().Get("Authorization"); strings.HasPrefix(value, "Bearer ") {
			token = strings.TrimPrefix(value, "Bearer ")
		}
	}
	valid, actor := s.checker.IsValidAccessToken(ctx, token, false)
	if !valid || actor == nil {
		return operatorV1.ErrorAdminUnauthorized("unauthorized")
	}
	if s.gateway == nil {
		return auth.ErrWrongContext
	}
	{
		var err error
		ctx, err = mw.ResolvePortal(ctx, s.gateway)
		if err != nil {
			return err
		}
	}
	if err := portalscope.CheckTenant(ctx, actor.GetTenantId()); err != nil {
		return err
	}
	if in.GetStream() != strconv.FormatUint(uint64(actor.GetUserId()), 10) {
		return operatorV1.ErrorAdminForbidden("stream user mismatch")
	}
	ctx = auth.NewContext(ctx, actor)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := s.client.Subscribe(ctx, &operatorV1.EventSubscriptionRequest{Stream: in.GetStream()})
	if err != nil {
		return err
	}
	for {
		frame, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := send(toAPIEventFrame(frame)); err != nil {
			return err
		}
	}
}

// NewExportHTTPService forwards typed export requests to operator.
func NewExportHTTPService(client operatorV1.ExportServiceClient) *ExportHTTPService {
	return &ExportHTTPService{client: client}
}

// NewMediaHTTPService forwards signed media requests to operator.
func NewMediaHTTPService(client operatorV1.MediaServiceClient) *MediaHTTPService {
	return &MediaHTTPService{client: client}
}

// NewEventHTTPService owns SSE authentication and its operator stream.
func NewEventHTTPService(client operatorV1.EventServiceClient, checker auth.AccessTokenChecker, gateway operatorV1.GatewayServiceClient) *EventHTTPService {
	return &EventHTTPService{
		client:  client,
		checker: checker,
		gateway: gateway,
	}
}
