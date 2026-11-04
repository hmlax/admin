package service

import (
	"testing"

	operatorV1 "go-admin/platform/pb/operator"

	"github.com/stretchr/testify/require"
	"github.com/tx7do/go-utils/trans"
)

// ---- renderTemplate 单元 ----

func TestRenderTemplate(t *testing.T) {
	t.Run("常规替换", func(t *testing.T) {
		got, err := renderTemplate("你好 {{name}}，订单 {{orderNo}} 已发货", map[string]string{
			"name": "张三", "orderNo": "A1024",
		})
		require.NoError(t, err)
		require.Equal(t, "你好 张三，订单 A1024 已发货", got)
	})

	t.Run("同名占位符多次出现", func(t *testing.T) {
		got, err := renderTemplate("{{x}}+{{x}}", map[string]string{"x": "1"})
		require.NoError(t, err)
		require.Equal(t, "1+1", got)
	})

	t.Run("占位符名两侧空白容忍", func(t *testing.T) {
		got, err := renderTemplate("hi {{ name }}", map[string]string{"name": "li"})
		require.NoError(t, err)
		require.Equal(t, "hi li", got)
	})

	t.Run("未识别占位符报错并带名字", func(t *testing.T) {
		_, err := renderTemplate("hi {{nmae}}", map[string]string{"name": "li"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "nmae")
	})

	t.Run("未闭合双括号按原文保留", func(t *testing.T) {
		got, err := renderTemplate(`JSON 示例 {"a": {{x}} }`, map[string]string{"x": "1"})
		require.NoError(t, err)
		require.Equal(t, `JSON 示例 {"a": 1 }`, got)
	})

	t.Run("纯文本不含占位符时变量集可为空", func(t *testing.T) {
		got, err := renderTemplate("纯文本", nil)
		require.NoError(t, err)
		require.Equal(t, "纯文本", got)
	})

	t.Run("空变量集下引用占位符报错（建模板时的语法试渲染路径）", func(t *testing.T) {
		_, err := renderTemplate("hi {{name}}", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "name")
	})

	t.Run("空占位符与未闭合按原文", func(t *testing.T) {
		got, err := renderTemplate("a {{}} b {{ c", nil)
		require.NoError(t, err)
		require.Equal(t, "a {{}} b {{ c", got)
	})
}

// ---- SendDirect 的 template_code 路径（SQLite 集成） ----

// TestSendDirectWithTemplate 钉住模板接入的四条语义：
//   - 命中启用模板：title/content 被渲染结果覆盖，渠道收到的就是渲染后的字节；
//   - 模板不存在：报错且不产生台账行（台账只记"确实要发的那一次"）；
//   - 模板停用：报错并区分于不存在（停用是运营动作，缺行是代码 bug）；
//   - 模板引用未知占位符：报错，同样零台账。
func TestSendDirectWithTemplate(t *testing.T) {
	env := newNotificationServiceForTest(t)
	ctx := env.ctx

	_, err := env.templateRepo.Create(ctx, &operatorV1.NotificationTemplate{
		Name:            trans.Ptr("发货通知"),
		Code:            trans.Ptr("order_shipped"),
		TitleTemplate:   trans.Ptr("订单 {{orderNo}} 已发货"),
		ContentTemplate: trans.Ptr("亲爱的 {{name}}，订单 {{orderNo}} 已由 {{carrier}} 发出"),
		IsEnabled:       trans.Ptr(true),
	}, 1)
	require.NoError(t, err, "建启用模板应成功")

	_, err = env.templateRepo.Create(ctx, &operatorV1.NotificationTemplate{
		Name:            trans.Ptr("停用模板"),
		Code:            trans.Ptr("disabled_tpl"),
		TitleTemplate:   trans.Ptr("t"),
		ContentTemplate: trans.Ptr("c"),
		IsEnabled:       trans.Ptr(false),
	}, 1)
	require.NoError(t, err, "建停用模板应成功")

	deliveriesBefore := func() int {
		n, err := env.client.NotificationDelivery.Query().Count(ctx)
		require.NoError(t, err)
		return n
	}

	// 命中：渲染覆盖 title/content，渠道收到渲染后的字节
	resp, err := env.svc.SendDirect(ctx, &operatorV1.SendDirectNotificationRequest{
		EventType:    operatorV1.EventType_PASSWORD_RESET_CODE,
		Target:       "user@example.com",
		TemplateCode: trans.Ptr("order_shipped"),
		TemplateVars: map[string]string{"orderNo": "A1024", "name": "张三", "carrier": "SF"},
	})
	require.NoError(t, err)
	require.Equal(t, operatorV1.DeliveryStatus_SENT, resp.GetStatus())
	require.Len(t, env.email.calls, 1)
	require.Equal(t, "订单 A1024 已发货", env.email.calls[0].Title, "渠道收到的应是渲染后的标题")
	require.Equal(t, "亲爱的 张三，订单 A1024 已由 SF 发出", env.email.calls[0].Content)

	baseline := deliveriesBefore()

	// 模板不存在：报错，零新台账
	_, err = env.svc.SendDirect(ctx, &operatorV1.SendDirectNotificationRequest{
		EventType:    operatorV1.EventType_PASSWORD_RESET_CODE,
		Target:       "user@example.com",
		TemplateCode: trans.Ptr("no_such_template"),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no_such_template")
	require.Equal(t, baseline, deliveriesBefore(), "模板解析失败不应产生台账行")

	// 停用模板：报错且文案区分于"不存在"
	_, err = env.svc.SendDirect(ctx, &operatorV1.SendDirectNotificationRequest{
		EventType:    operatorV1.EventType_PASSWORD_RESET_CODE,
		Target:       "user@example.com",
		TemplateCode: trans.Ptr("disabled_tpl"),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "disabled")
	require.Equal(t, baseline, deliveriesBefore())

	// 未知占位符：报错，零新台账
	_, err = env.svc.SendDirect(ctx, &operatorV1.SendDirectNotificationRequest{
		EventType:    operatorV1.EventType_PASSWORD_RESET_CODE,
		Target:       "user@example.com",
		TemplateCode: trans.Ptr("order_shipped"),
		TemplateVars: map[string]string{"orderNo": "A1024", "name": "张三"}, // 缺 carrier
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "carrier")
	require.Equal(t, baseline, deliveriesBefore())
}
