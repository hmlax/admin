package service

import (
	"context"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bLogger "github.com/tx7do/kratos-bootstrap/logger"
	"google.golang.org/protobuf/types/known/emptypb"

	paginationV1 "github.com/tx7do/go-crud/api/gen/go/pagination/v1"

	"go-admin/operator/internal/data"

	operatorV1 "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/middleware/auth"
)

type AiMessageService struct {
	operatorV1.UnimplementedAiMessageServiceServer
	log  *bLogger.Helper
	repo *data.AiMessageRepo
}

func NewAiMessageService(ctx *bootstrap.Context, repo *data.AiMessageRepo) *AiMessageService {
	return &AiMessageService{
		log:  ctx.NewLoggerHelper("ai_message/service/admin-service"),
		repo: repo,
	}
}

// List 消息列表按 filter 里的 conversationId 查询；repo 层强制 user_id=当前用户，
// 无论 filter 怎么拼都读不到他人的消息。
func (s *AiMessageService) List(ctx context.Context, req *paginationV1.PagingRequest) (*operatorV1.ListAiMessageResponse, error) {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, req, operator.UserId)
}

// Delete 仅能删除自己的消息（message.user_id == 当前用户）。
func (s *AiMessageService) Delete(ctx context.Context, req *operatorV1.DeleteAiMessageRequest) (*emptypb.Empty, error) {
	operator, err := auth.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	entity, err := s.repo.GetEntityByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if entity.UserID == nil || *entity.UserID != operator.UserId {
		return nil, operatorV1.ErrorAdminNotFound("ai message not found")
	}

	if err = s.repo.Delete(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
