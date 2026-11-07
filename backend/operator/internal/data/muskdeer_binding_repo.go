package data

import (
	"context"
 "errors"
	"math"
	"time"

	"go-admin/operator/internal/data/store"
	binding "go-admin/operator/internal/data/store/muskdeeraccountbinding"
	"go-admin/operator/internal/data/store/tenant"
	pb "go-admin/platform/pb/operator"
	"go-admin/platform/pkg/middleware/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const muskdeerScopeDenied = "a bound trading account is required"

type MuskdeerBindingRepo struct{ db *store.DBClient }

func NewMuskdeerBindingRepo(db *store.DBClient) *MuskdeerBindingRepo {
	return &MuskdeerBindingRepo{db: db}
}

// MuskdeerIdentity is derived only from the trusted gateway context.
func MuskdeerIdentity(ctx context.Context, platform bool) (*pb.UserTokenPayload, error) {
	actor, err := auth.FromContext(ctx)
	if err != nil || actor.GetUserId() == 0 || platform && actor.GetTenantId() != 0 {
		return nil, status.Error(codes.PermissionDenied, "trusted administration context required")
	}
	return actor, nil
}

// Scope is fetched on every call, so disabling a binding revokes the next request.
func (r *MuskdeerBindingRepo) Scope(ctx context.Context) ([]string, error) {
	actor, err := MuskdeerIdentity(ctx, false)
	if err != nil {
		return nil, err
	}
	if actor.GetTenantId() == 0 {
		return nil, nil
	}
	rows, err := r.db.Client().MuskdeerAccountBinding.Query().Where(binding.TenantIDEQ(actor.GetTenantId()), binding.IsEnabledEQ(true), binding.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.AccountID != nil && *row.AccountID != "" {
			result = append(result, *row.AccountID)
		}
	}
	if len(result) == 0 {
		return nil, status.Error(codes.PermissionDenied, muskdeerScopeDenied)
	}
	return result, nil
}

func bindingDTO(row *store.MuskdeerAccountBinding) *pb.MuskdeerAccountBinding {
	return &pb.MuskdeerAccountBinding{
		Id:        row.ID,
		TenantId:  valueOrZero(row.TenantID),
		AccountId: valueOrZero(row.AccountID),
		Enabled:   valueOrZero(row.IsEnabled),
		Version:   valueOrZero(row.Version),
	}
}
func valueOrZero[T any](v *T) T {
	if v != nil {
		return *v
	}
	var zero T
	return zero
}

func (r *MuskdeerBindingRepo) List(ctx context.Context, in *pb.MuskdeerBindingListRequest) (*pb.MuskdeerBindingPage, error) {
	if _, err := MuskdeerIdentity(ctx, true); err != nil {
		return nil, err
	}
	page, size := in.GetPage(), in.GetPageSize()
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if size > 200 || page > 10000000 {
		return nil, status.Error(codes.InvalidArgument, "invalid pagination")
	}
	q := r.db.Client().MuskdeerAccountBinding.Query().Where(binding.DeletedAtIsNil())
	if in.GetTenantId() > 0 {
		q = q.Where(binding.TenantIDEQ(in.TenantId))
	}
	if in.GetAccountId() != "" {
		q = q.Where(binding.AccountIDEQ(in.AccountId))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.Order(binding.ByID()).Offset(int(page-1) * int(size)).Limit(int(size)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := &pb.MuskdeerBindingPage{Total: uint64(total)}
	for _, row := range rows {
		out.Items = append(out.Items, bindingDTO(row))
	}
	return out, nil
}

func (r *MuskdeerBindingRepo) Create(ctx context.Context, in *pb.MuskdeerBindingDraft) (*pb.MuskdeerAccountBinding, error) {
	actor, err := MuskdeerIdentity(ctx, true)
	if err != nil {
		return nil, err
	}
	if in.GetTenantId() == 0 || in.GetAccountId() == "" || len(in.GetAccountId()) > 64 {
		return nil, status.Error(codes.InvalidArgument, "tenant and trading account required")
	}
	exists, err := r.db.Client().Tenant.Query().Where(tenant.IDEQ(in.TenantId), tenant.DeletedAtIsNil()).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}
	now := time.Now().UTC()
	row, err := r.db.Client().MuskdeerAccountBinding.Create().SetTenantID(in.TenantId).SetAccountID(in.AccountId).SetIsEnabled(in.Enabled).SetVersion(1).SetCreatedBy(actor.UserId).SetUpdatedBy(actor.UserId).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
	if err != nil {
		return nil, err
	}
	return bindingDTO(row), nil
}

func (r *MuskdeerBindingRepo) SetEnabled(ctx context.Context, in *pb.MuskdeerSetBindingEnabledRequest) (*pb.MuskdeerAccountBinding, error) {
	actor, err := MuskdeerIdentity(ctx, true)
	if err != nil {
		return nil, err
	}
	if in.GetId() == 0 || in.GetExpectedVersion() == 0 || in.GetExpectedVersion() >= math.MaxInt64 {
		return nil, status.Error(codes.InvalidArgument, "invalid binding version")
	}
	count, err := r.db.Client().MuskdeerAccountBinding.Update().Where(binding.IDEQ(in.Id), binding.VersionEQ(in.ExpectedVersion), binding.DeletedAtIsNil()).SetIsEnabled(in.Enabled).SetVersion(in.ExpectedVersion + 1).SetUpdatedBy(actor.UserId).SetUpdatedAt(time.Now().UTC()).Save(ctx)
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, status.Error(codes.Aborted, "binding changed; refresh before retrying")
	}
	row, err := r.db.Client().MuskdeerAccountBinding.Query().Where(binding.IDEQ(in.Id)).Only(ctx)
	if err != nil {
		return nil, err
	}
	return bindingDTO(row), nil
}

// CheckSchema rejects enablement before the separate administration migration.
func(r *MuskdeerBindingRepo) CheckSchema(ctx context.Context)error{
 migrator:=r.db.Client().GormDB().WithContext(ctx).Migrator()
 for _,column:=range []string{"id","created_at","updated_at","deleted_at","tenant_id","account_id","is_enabled","version"}{
  if !migrator.HasColumn(&store.MuskdeerAccountBinding{},column){return errors.New("Muskdeer administration schema v1 required; run muskdeer-migrate before enabling")}
 }
 if !migrator.HasIndex(&store.MuskdeerAccountBinding{},"uk_muskdeer_binding_account"){return errors.New("Muskdeer account ownership unique index required")}
 return nil
}
