package server

import (
	"context"

	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/middleware/auth"

	"google.golang.org/grpc"
)

// ExportRPCService exposes typed export operations owned by operator.
type ExportRPCService struct {
	operatorV1.UnimplementedExportServiceServer
	gateway *GatewayService
}

func (s *ExportRPCService) ExportAuditLogs(ctx context.Context, in *operatorV1.AuditExportRequest) (*operatorV1.BinaryFileResponse, error) {
	return s.export(ctx, &operatorV1.ExportRequest{
		Resource: operatorV1.ExportRequest_AUDIT,
		Type:     in.GetType(),
		Format:   in.GetFormat(),
		MaxRows:  in.GetMaxRows(),
		Query:    in.GetQuery(),
	}, true)
}

func (s *ExportRPCService) ExportAIUsage(ctx context.Context, in *operatorV1.ExportFileRequest) (*operatorV1.BinaryFileResponse, error) {
	return s.export(ctx, &operatorV1.ExportRequest{
		Resource: operatorV1.ExportRequest_AI_USAGE,
		Format:   in.GetFormat(),
		MaxRows:  in.GetMaxRows(),
		Query:    in.GetQuery(),
	}, false)
}

func (s *ExportRPCService) ExportNotificationDeliveries(ctx context.Context, in *operatorV1.ExportFileRequest) (*operatorV1.BinaryFileResponse, error) {
	return s.export(ctx, &operatorV1.ExportRequest{
		Resource: operatorV1.ExportRequest_NOTIFICATION_DELIVERY,
		Format:   in.GetFormat(),
		MaxRows:  in.GetMaxRows(),
		Query:    in.GetQuery(),
	}, true)
}

func (s *ExportRPCService) export(ctx context.Context, in *operatorV1.ExportRequest, platformOnly bool) (*operatorV1.BinaryFileResponse, error) {
	actor, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if platformOnly && !actor.GetIsPlatformAdmin() {
		return nil, operatorV1.ErrorAdminForbidden(errorPlatformExportRequired)
	}
	response, err := s.gateway.Export(ctx, in)
	if err != nil {
		return nil, err
	}
	return rpcFileResponse(response), nil
}

const errorPlatformExportRequired = "platform administrator required"

func rpcFileResponse(in *operatorV1.FileResponse) *operatorV1.BinaryFileResponse {
	return &operatorV1.BinaryFileResponse{
		Content:      in.GetContent(),
		ContentType:  in.GetContentType(),
		Disposition:  in.GetDisposition(),
		Status:       in.GetStatus(),
		CacheControl: in.GetCacheControl(),
	}
}

type MediaRPCService struct {
	operatorV1.UnimplementedMediaServiceServer
	gateway *GatewayService
}

func (s *MediaRPCService) ReadImage(ctx context.Context, in *operatorV1.ReadMediaRequest) (*operatorV1.BinaryFileResponse, error) {
	response, err := s.gateway.ReadMedia(ctx, &operatorV1.MediaRequest{
		Path:      in.GetPath(),
		Expires:   in.GetExpires(),
		Signature: in.GetSig(),
	})
	if err != nil {
		return nil, err
	}
	return rpcFileResponse(response), nil
}

type EventRPCService struct {
	operatorV1.UnimplementedEventServiceServer
	gateway *GatewayService
}

func (s *EventRPCService) Subscribe(in *operatorV1.EventSubscriptionRequest, stream grpc.ServerStreamingServer[operatorV1.EventFrame]) error {
	return s.gateway.SubscribeEvents(&operatorV1.SubscribeRequest{Stream: in.GetStream()}, eventFrameStream{ServerStreamingServer: stream})
}

type eventFrameStream struct {
	grpc.ServerStreamingServer[operatorV1.EventFrame]
}

func (s eventFrameStream) Send(in *operatorV1.Event) error {
	return s.ServerStreamingServer.Send(&operatorV1.EventFrame{
		Id:      in.GetId(),
		Data:    in.GetData(),
		Event:   in.GetEvent(),
		Retry:   in.GetRetry(),
		Comment: in.GetComment(),
	})
}
