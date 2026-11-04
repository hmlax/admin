package server

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tx7do/go-crud/viewer"
	"github.com/tx7do/go-utils/authorizer"
	"github.com/tx7do/kratos-transport/transport/sse"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"go-admin/operator/internal/data"
	"go-admin/operator/internal/service"
	operatorV1 "go-admin/platform/pb/operator"
)

// GatewayService exposes typed data operations needed by the HTTP boundary.
type GatewayService struct {
	operatorV1.UnimplementedGatewayServiceServer
	Policies        authorizer.Provider
	Access          *data.GatewayAccessRepo
	APIAudit        *data.ApiAuditLogRepo
	LoginAudit      *data.LoginAuditLogRepo
	OperationAudit  *data.OperationAuditLogRepo
	PermissionAudit *data.PermissionAuditLogRepo
	DataAccessAudit *data.DataAccessAuditLogRepo
	PolicyAudit     *data.PolicyEvaluationLogRepo
	AuditExport     *service.AuditExportService
	DataExport      *service.DataExportService
	Files           *service.FileTransferService
	Redis           *redis.Client
}

func (s *GatewayService) GetPolicies(ctx context.Context, _ *emptypb.Empty) (*operatorV1.PoliciesResponse, error) {
	policies, err := s.Policies.ProvidePolicies(ctx)
	if err != nil {
		return nil, err
	}
	out := &operatorV1.PoliciesResponse{Policies: make([]*operatorV1.RolePolicy, 0, len(policies))}
	for role, permissions := range policies {
		entry := &operatorV1.RolePolicy{Role: role}
		for _, p := range permissions {
			entry.Permissions = append(entry.Permissions, &operatorV1.GatewayPermission{
				Domain: p.Domain,
				Path:   p.Path,
				Method: p.Method,
			})
		}
		out.Policies = append(out.Policies, entry)
	}
	return out, nil
}
func (s *GatewayService) GetTenantAccessData(ctx context.Context, req *operatorV1.TenantAccessRequest) (*operatorV1.TenantAccessData, error) {
	return s.Access.Get(ctx, req)
}

func (s *GatewayService) ResolveTenantCode(ctx context.Context, req *operatorV1.ResolveTenantCodeRequest) (*operatorV1.ResolveTenantCodeResponse, error) {
	return s.Access.ResolveTenantCode(ctx, req.GetCode())
}
func (s *GatewayService) WriteAudit(ctx context.Context, req *operatorV1.AuditRecord) (*emptypb.Empty, error) {
	ctx = viewer.WithSystemContext(ctx)
	var err error
	switch record := req.GetRecord().(type) {
	case *operatorV1.AuditRecord_Api:
		err = s.APIAudit.Create(ctx, &operatorV1.CreateApiAuditLogRequest{Data: record.Api})
	case *operatorV1.AuditRecord_Login:
		err = s.LoginAudit.Create(ctx, &operatorV1.CreateLoginAuditLogRequest{Data: record.Login})
	case *operatorV1.AuditRecord_Operation:
		err = s.OperationAudit.Create(ctx, &operatorV1.CreateOperationAuditLogRequest{Data: record.Operation})
	case *operatorV1.AuditRecord_Permission:
		err = s.PermissionAudit.Create(ctx, &operatorV1.CreatePermissionAuditLogRequest{Data: record.Permission})
	case *operatorV1.AuditRecord_DataAccess:
		err = s.DataAccessAudit.Create(ctx, &operatorV1.CreateDataAccessAuditLogRequest{Data: record.DataAccess})
	case *operatorV1.AuditRecord_PolicyEvaluation:
		err = s.PolicyAudit.Create(ctx, &operatorV1.CreatePolicyEvaluationLogRequest{Data: record.PolicyEvaluation})
	default:
		return nil, fmt.Errorf("audit record is required")
	}
	return &emptypb.Empty{}, err
}
func (s *GatewayService) ResolvePolicy(ctx context.Context, req *operatorV1.ResolvePolicyRequest) (*operatorV1.ResolvePolicyResponse, error) {
	permission, policy := s.PolicyAudit.ResolvePermissionPolicyByRoute(viewer.WithSystemContext(ctx), req.GetSubject(), req.GetPath(), req.GetMethod())
	return &operatorV1.ResolvePolicyResponse{
		PermissionId: permission,
		PolicyId:     policy,
	}, nil
}

type fileWriter struct {
	bytes.Buffer
	header http.Header
	status int
}

func (w *fileWriter) Header() http.Header { return w.header }
func (w *fileWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *fileWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.Buffer.Write(data)
}
func (w *fileWriter) response() *operatorV1.FileResponse {
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	return &operatorV1.FileResponse{
		Content:      w.Bytes(),
		ContentType:  w.header.Get("Content-Type"),
		Disposition:  w.header.Get("Content-Disposition"),
		Status:       int32(status),
		CacheControl: w.header.Get("Cache-Control"),
	}
}
func (s *GatewayService) Export(ctx context.Context, req *operatorV1.ExportRequest) (*operatorV1.FileResponse, error) {
	q := url.Values{"type": {req.GetType()}, "format": {req.GetFormat()}, "maxRows": {strconv.FormatUint(uint64(req.GetMaxRows()), 10)}, "query": {req.GetQuery()}}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://admin/export?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	w := &fileWriter{header: make(http.Header)}
	switch req.GetResource() {
	case operatorV1.ExportRequest_AUDIT:
		err = s.AuditExport.ServeExport(w, r)
	case operatorV1.ExportRequest_AI_USAGE:
		err = s.DataExport.ServeAiUsageExport(w, r)
	case operatorV1.ExportRequest_NOTIFICATION_DELIVERY:
		err = s.DataExport.ServeNotificationDeliveryExport(w, r)
	default:
		return nil, fmt.Errorf("unknown export resource")
	}
	if err != nil {
		return nil, err
	}
	return w.response(), nil
}
func (s *GatewayService) ReadMedia(ctx context.Context, req *operatorV1.MediaRequest) (*operatorV1.FileResponse, error) {
	q := url.Values{"path": {req.GetPath()}, "expires": {strconv.FormatInt(req.GetExpires(), 10)}, "sig": {req.GetSignature()}}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://admin/image?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	w := &fileWriter{header: make(http.Header)}
	if err := s.Files.ServeImageHandler(w, r); err != nil {
		return nil, err
	}
	return w.response(), nil
}

const eventChannelPrefix = "go-admin:sse:"

func (s *GatewayService) Publish(ctx context.Context, stream sse.StreamID, event *sse.Event) {
	s.TryPublish(ctx, stream, event)
}
func (s *GatewayService) TryPublish(ctx context.Context, stream sse.StreamID, event *sse.Event) bool {
	data, err := proto.Marshal(&operatorV1.Event{
		Id:      event.ID,
		Data:    event.Data,
		Event:   event.Event,
		Retry:   event.Retry,
		Comment: event.Comment,
	})
	if err != nil {
		slog.ErrorContext(ctx, "encode SSE event", "error", err)
		return false
	}
	result := s.Redis.Publish(ctx, eventChannelPrefix+string(stream), data)
	if result.Err() != nil {
		slog.ErrorContext(ctx, "publish SSE event", "error", result.Err())
		return false
	}
	return result.Val() > 0
}
func (s *GatewayService) SubscribeEvents(req *operatorV1.SubscribeRequest, stream operatorV1.GatewayService_SubscribeEventsServer) error {
	ctx := stream.Context()
	subscription := s.Redis.Subscribe(ctx, eventChannelPrefix+req.GetStream())
	defer func() {
		if closeErr := subscription.Close(); closeErr != nil {
			slog.Error("close gateway subscription", "error", closeErr)
		}
	}()
	if _, err := subscription.Receive(ctx); err != nil {
		return err
	}
	// Initial comment lets the gateway finish headers after subscription is ready.
	if err := stream.Send(&operatorV1.Event{Comment: []byte("connected")}); err != nil {
		return err
	}
	// PubSub owns its receive goroutine; Close above stops it on cancellation.
	messages := subscription.Channel(redis.WithChannelSize(1))
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-heartbeat.C:
			if err := stream.Send(&operatorV1.Event{Comment: []byte("heartbeat")}); err != nil {
				return err
			}
		case message, ok := <-messages:
			if !ok {
				return fmt.Errorf("SSE event subscription closed")
			}
			event := new(operatorV1.Event)
			if err := proto.Unmarshal([]byte(message.Payload), event); err != nil {
				return err
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}
