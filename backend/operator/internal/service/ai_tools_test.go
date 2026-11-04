package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/require"
)

// startFakeOpenAI 起一个 OpenAI 兼容的流式假服务：
// 第 1 次请求（带 tools）→ 返回 tool_calls 增量流（finish_reason=tool_calls）；
// 之后的请求 → 返回纯文本增量流（finish_reason=stop）。
// 记录每次请求的 body 供断言（第二轮必须含 tool 角色消息）。
func startFakeOpenAI(t *testing.T) *httptest.Server {
	t.Helper()
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Logf("fake openai got unmatched request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(404)
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		if callCount == 1 {
			// 第一轮：请求工具调用（增量 name/arguments + finish_reason=tool_calls）
			writeChunk := func(payload map[string]any) {
				raw, _ := json.Marshal(payload)
				_, _ = w.Write([]byte("data: " + string(raw) + "\n\n"))
				flusher.Flush()
			}
			writeChunk(map[string]any{
				"choices": []any{map[string]any{
					"delta": map[string]any{
						"tool_calls": []any{map[string]any{
							"index": 0, "id": "call_1", "type": "function",
							"function": map[string]any{"name": "get_current_time", "arguments": ""},
						}},
					},
				}},
			})
			writeChunk(map[string]any{
				"choices": []any{map[string]any{
					"delta": map[string]any{
						"tool_calls": []any{map[string]any{
							"index": 0, "function": map[string]any{"arguments": "{}"},
						}},
					},
					"finish_reason": "tool_calls",
				}},
			})
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}

		// 第二轮及以后：纯文本增量
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		raw, _ := json.Marshal(body)
		t.Logf("round %d body: %s", callCount, string(raw))

		writeChunk := func(content string, finish any) {
			payload := map[string]any{
				"choices": []any{map[string]any{
					"delta":         map[string]any{"content": content},
					"finish_reason": finish,
				}},
			}
			raw, _ := json.Marshal(payload)
			_, _ = w.Write([]byte("data: " + string(raw) + "\n\n"))
			flusher.Flush()
		}
		writeChunk("当前时间是 ", nil)
		writeChunk("2026-10-03 12:00:00", "stop")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})

	return httptest.NewServer(mux)
}

// TestAiToolLoopToolCallRoundTrip Function Call 协议层往返：
// 模型请求工具 → get_current_time 被执行 → 结果回传 → 第二轮给出文本答案。
func TestAiToolLoopToolCallRoundTrip(t *testing.T) {
	srv := startFakeOpenAI(t)
	defer srv.Close()

	cfg := openai.DefaultConfig("")
	cfg.BaseURL = srv.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)

	var deltas []string
	var toolCalls [][3]string // [name, arguments, result]
	runner := &aiToolLoop{
		client:    client,
		model:     "test-model",
		tools:     aiBuiltinToolDefs(),
		exec:      execAiTool,
		maxRounds: aiToolMaxRounds,
		onDelta:   func(delta string) { deltas = append(deltas, delta) },
		onToolCall: func(name, arguments, result string) {
			toolCalls = append(toolCalls, [3]string{name, arguments, result})
		},
	}

	full, usage, err := runner.run(context.Background(), []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "现在几点了"},
	})
	require.NoError(t, err)
	require.Equal(t, "当前时间是 2026-10-03 12:00:00", full, "最终答案应拼接自文本增量")
	require.Len(t, deltas, 2, "文本增量应经 onDelta 推送")

	// 工具调用可见化回调：一次调用，参数与执行结果齐全（结果为服务端时间文本）
	require.Len(t, toolCalls, 1, "工具执行完成应经 onToolCall 回调一帧")
	require.Equal(t, "get_current_time", toolCalls[0][0])
	require.Equal(t, "{}", toolCalls[0][1])
	require.Regexp(t, `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`, toolCalls[0][2])

	// 累计用量字段存在（假服务不带 usage，保持零值即可）
	require.Zero(t, usage.TotalTokens)
}

// TestAiToolLoopMaxRounds 模型持续请求工具（永不给文本）→ 轮数耗尽报错，
// 且最后一轮请求不带 tools（强制给文本）。
func TestAiToolLoopMaxRounds(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		raw, _ := json.Marshal(map[string]any{
			"choices": []any{map[string]any{
				"delta": map[string]any{
					"tool_calls": []any{map[string]any{
						"index": 0, "id": "call_x", "type": "function",
						"function": map[string]any{"name": "get_current_time", "arguments": "{}"},
					}},
				},
				"finish_reason": "tool_calls",
			}},
		})
		_, _ = w.Write([]byte("data: " + string(raw) + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer srv.Close()

	cfg := openai.DefaultConfig("")
	cfg.BaseURL = srv.URL + "/v1"
	client := openai.NewClientWithConfig(cfg)
	runner := &aiToolLoop{
		client: client, model: "test-model", tools: aiBuiltinToolDefs(),
		exec: execAiTool, maxRounds: 3,
	}
	_, _, err := runner.run(context.Background(), []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleUser, Content: "现在几点"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "without a final answer")
	require.Len(t, bodies, 3, "每轮都应发起请求")
	// 最后一轮强制不带 tools
	lastTools, hasTools := bodies[len(bodies)-1]["tools"]
	require.False(t, hasTools && lastTools != nil, "最后一轮请求不应携带 tools")
}
