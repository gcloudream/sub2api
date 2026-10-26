package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayForwardCodexImageLiteCompatibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, prompt string
		wantImage    bool
	}{
		{"image turn upgrades without client catalog", "请生成一张窗边橘猫的图片", true},
		{"ordinary turn retains Lite", "请解释这个函数为什么返回空值", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := `{"id":"resp_test","status":"completed","model":"gpt-6.1-sol","output":[],"usage":{"input_tokens":1,"output_tokens":2}}`
			if tc.wantImage {
				response = `{"id":"resp_test","status":"completed","model":"gpt-6.1-sol","output":[{"id":"ig_test","type":"image_generation_call","status":"completed","result":"aGVsbG8=","size":"1024x1024"}],"usage":{"input_tokens":1,"output_tokens":2}}`
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(response)),
			}}
			cfg := &config.Config{}
			cfg.Gateway.CodexImageGenerationBridgeEnabled = true
			svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream}
			account := &Account{ID: 7, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
				Concurrency: 1, Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"access_token": "test-oauth-token", "chatgpt_account_id": "test-account"}}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
			c.Request.Header.Set("User-Agent", "codex_cli_rs/0.160.0")
			c.Request.Header.Set("originator", "codex_cli_rs")
			c.Request.Header.Set(responsesLiteHeader, "true")
			c.Set("api_key", &APIKey{ID: 3, Group: &Group{AllowImageGeneration: true}})
			SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
			body := []byte(`{"model":"gpt-6.1-sol","stream":false,"tool_choice":"auto","input":[{"type":"additional_tools","role":"developer","tools":[{"type":"namespace","name":"example","tools":[{"type":"function","name":"lookup","parameters":{"type":"object","properties":{}}}]}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"` + tc.prompt + `"}]}],"reasoning":{"effort":"low","context":"all_turns"},"client_metadata":{"keep":"yes","ws_request_header_x_openai_internal_codex_responses_lite":"true"}}`)

			result, err := svc.Forward(context.Background(), c, account, body)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, upstream.lastReq)
			require.Equal(t, !tc.wantImage, isOpenAIResponsesLiteHeader(upstream.lastReq.Header.Get(responsesLiteHeader)))
			require.Equal(t, !tc.wantImage, isOpenAIResponsesLiteWebSocketPayload(upstream.lastBody))
			require.Equal(t, tc.wantImage, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
			require.Equal(t, tc.wantImage, strings.Contains(gjson.GetBytes(upstream.lastBody, "instructions").String(), codexImageGenerationBridgeMarker))
			require.Equal(t, tc.wantImage, codexImageDeliveryInputMarkers(t, upstream.lastBody) == 1)
			require.Equal(t, tc.wantImage, codexImageDeliveryInputEndsWithMarker(upstream.lastBody))
			require.Equal(t, "auto", gjson.GetBytes(upstream.lastBody, "tool_choice").String())
			require.Contains(t, string(upstream.lastBody), tc.prompt)
			require.Equal(t, "true", c.GetHeader(responsesLiteHeader), "failover must retain ingress protocol")
			require.True(t, isOpenAIResponsesLiteWebSocketPayload(body), "caller bytes must remain untouched")
			if tc.wantImage {
				require.Equal(t, 1, result.ImageCount)
			}
		})
	}
}
