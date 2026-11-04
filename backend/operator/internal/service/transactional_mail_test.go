package service

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"

	"go-admin/operator/internal/data"
	"go-admin/platform/pkg/mailtext"

	storetest "go-admin/operator/internal/data/storetest"
	operatorV1 "go-admin/platform/pb/operator"
)

// newTplMailerForTest 构造带真模板仓储的 mailer（sqlite 内存库）。
func newTplMailerForTest(t *testing.T) (*TransactionMailer, context.Context) {
	t.Helper()
	dbClient := storetest.NewClientForTest(t)
	repo := data.NewNotificationTemplateRepoForTest(dbClient)
	ctx := storetest.NewSystemContext(context.Background())
	return &TransactionMailer{TemplateRepo: repo}, ctx
}

func seedTemplate(t *testing.T, mailer *TransactionMailer, ctx context.Context, code, titleTpl, contentTpl string) {
	t.Helper()
	_, err := mailer.TemplateRepo.Create(ctx, &operatorV1.NotificationTemplate{
		Name:            trans.Ptr("tpl " + code),
		Code:            trans.Ptr(code),
		TitleTemplate:   trans.Ptr(titleTpl),
		ContentTemplate: trans.Ptr(contentTpl),
		IsEnabled:       trans.Ptr(true),
	}, 1)
	require.NoError(t, err)
}

func TestRenderPwdResetCodeFallback(t *testing.T) {
	mailer, ctx := newTplMailerForTest(t)
	title, content := renderPwdResetCode(ctx, mailer, "ABC123")
	require.NotEmpty(t, title)
	require.Contains(t, content, "ABC123", "回落文案应含验证码")
}

// TestRenderPwdResetCodeTemplateOverride 模板命中：按 code 覆写文案，
// {{code}}/{{codeSpaced}} 变量被渲染。
func TestRenderPwdResetCodeTemplateOverride(t *testing.T) {
	mailer, ctx := newTplMailerForTest(t)
	seedTemplate(t, mailer, ctx, TplCodePwdReset,
		"重置您的密码",
		"验证码：{{code}}（分隔 {{codeSpaced}}）。10 分钟内有效。")

	title, content := renderPwdResetCode(ctx, mailer, "AB CD")
	require.Equal(t, "重置您的密码", title)
	require.Equal(t, "验证码：AB CD（分隔 A B   C D）。10 分钟内有效。", content)
}

// TestRenderTemplateDisabledFallsBack 模板停用 → 回落内置文案。
func TestRenderTemplateDisabledFallsBack(t *testing.T) {
	mailer, ctx := newTplMailerForTest(t)
	seedTemplate(t, mailer, ctx, TplCodeContactBind,
		"停用模板标题", "停用模板内容 {{code}}")
	// 停用：更新 isEnabled=false
	tpl, err := mailer.TemplateRepo.GetByCode(ctx, TplCodeContactBind)
	require.NoError(t, err)
	require.NoError(t, mailer.TemplateRepo.Update(ctx, &operatorV1.UpdateNotificationTemplateRequest{
		Id:         tpl.GetId(),
		Data:       &operatorV1.NotificationTemplate{IsEnabled: trans.Ptr(false)},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"isEnabled"}},
	}, 1))

	title, content := renderContactBindCode(ctx, mailer, "XYZ789")
	require.Contains(t, content, "XYZ789", "停用模板应回落内置文案且仍带验证码")
	_ = title
}

// TestRenderChannelTestEmailTemplate 渠道测试邮件模板覆写。
func TestRenderChannelTestEmailTemplate(t *testing.T) {
	mailer, ctx := newTplMailerForTest(t)
	seedTemplate(t, mailer, ctx, TplCodeChannelTest,
		"渠道 {{channelId}} 测试",
		"操作人 {{operatorId}} 发起的测试邮件。")

	title, content := renderChannelTestEmail(ctx, mailer, 42, 7)
	require.Equal(t, "渠道 42 测试", title)
	require.Equal(t, "操作人 7 发起的测试邮件。", content)
}

// TestRenderRespectsWithLocale 邮件语言显式指定：mailtext.WithLocale 注入后
// 内置文案走对应语言表（模板缺失回落路径）。
func TestRenderRespectsWithLocale(t *testing.T) {
	dbClient := storetest.NewClientForTest(t)
	mailer := &TransactionMailer{TemplateRepo: data.NewNotificationTemplateRepoForTest(dbClient)}
	ctx := storetest.NewSystemContext(context.Background())

	// en locale + 无模板 → mailtext 内置英文文案
	ctxEn := mailtext.WithLocale(ctx, mailtext.LocaleEnUS)
	title, content := renderPwdResetCode(ctxEn, mailer, "ABC123")
	require.Contains(t, title, "password reset", "en locale 应回落英文内置文案")
	require.Contains(t, content, "ABC123")

	// 默认（无显式 locale）→ 中文
	titleZh, _ := renderPwdResetCode(ctx, mailer, "ABC123")
	require.Contains(t, titleZh, "重置", "默认 zh-CN 内置文案")
}
