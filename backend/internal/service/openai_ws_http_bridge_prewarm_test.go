package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIWSHTTPBridgePrewarmRequiresBooleanFalse(t *testing.T) {
	for _, tt := range []struct {
		payload string
		want    bool
	}{
		{`{"generate":false}`, true},
		{`{"generate":true}`, false},
		{`{"generate":"false"}`, false},
		{`{"generate":0}`, false},
		{`{"generate":null}`, false},
		{`{}`, false},
	} {
		require.Equal(t, tt.want, isOpenAIWSHTTPBridgePrewarm([]byte(tt.payload)), tt.payload)
	}
	_, err := prepareOpenAIWSHTTPBridgeBody(nil, []byte(`{"type":"response.create","generate":false,"model":"gpt-6.1-sol","input":"warm"}`))
	require.ErrorContains(t, err, "must be acknowledged locally")
}

func TestProxyOpenAIWSHTTPBridgeTurnPrewarmAcknowledgesLocally(t *testing.T) {
	for _, failAt := range []int{-1, 0, 1} {
		t.Run(fmt.Sprintf("write_failure_at_%d", failAt), func(t *testing.T) {
			payload := []byte(`{"type":"response.create","generate":false,"model":"gpt-6.1-sol","input":[{"role":"developer","content":"keep warm context"}]}`)
			original := append([]byte(nil), payload...)
			var writes [][]byte
			writeErr := errors.New("client disconnected during prewarm")
			// Deliberately omit httpUpstream: acknowledging a client prewarm
			// must not even depend on the model transport being available.
			svc := &OpenAIGatewayService{}
			result, err := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), nil, &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}, "test-token", payload, len(payload), "gpt-6.1-sol", "", "", "", "", 1, func(message []byte) error {
				index := len(writes)
				writes = append(writes, append([]byte(nil), message...))
				if index == failAt {
					return writeErr
				}
				return nil
			})
			require.Equal(t, original, payload)
			if failAt >= 0 {
				require.ErrorIs(t, err, writeErr)
				require.Nil(t, result)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, writes, 2)
			require.True(t, strings.HasPrefix(result.RequestID, "resp_sub2api_prewarm_"))
			require.Equal(t, "response.created", gjson.GetBytes(writes[0], "type").String())
			require.Equal(t, "response.completed", gjson.GetBytes(writes[1], "type").String())
			require.Equal(t, result.RequestID, gjson.GetBytes(writes[0], "response.id").String())
			require.Equal(t, result.RequestID, gjson.GetBytes(writes[1], "response.id").String())
			require.Equal(t, "completed", gjson.GetBytes(writes[1], "response.status").String())
			require.Equal(t, "[]", gjson.GetBytes(writes[1], "response.output").Raw)
			require.Zero(t, gjson.GetBytes(writes[1], "response.usage.input_tokens").Int())
			require.Zero(t, gjson.GetBytes(writes[1], "response.usage.output_tokens").Int())
			require.Zero(t, gjson.GetBytes(writes[1], "response.usage.total_tokens").Int())
		})
	}
}

func TestOpenAIWSHTTPBridgeClientPrewarmReplaysWithoutBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	completed := func(id, output string) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf("data: {\"type\":\"response.completed\",\"response\":{\"id\":%q,\"status\":\"completed\",\"model\":\"gpt-6.1-sol\",\"output\":%s,\"usage\":{\"input_tokens\":3,\"output_tokens\":2}}}\n\n", id, output)))}
	}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		completed("resp_prewarm_business_image", `[{"id":"ig_real","type":"image_generation_call","status":"completed","result":"aW1hZ2U=","size":"1024x1024"}]`),
		completed("resp_prewarm_business_text", `[]`),
	}}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector()}
	account := &Account{ID: 9901, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
		Extra:       map[string]any{"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeHTTPBridge, "openai_oauth_responses_websockets_v2_enabled": false, "codex_image_generation_bridge": true},
	}
	type afterTurn struct {
		turn   int
		result *OpenAIForwardResult
		err    error
	}
	after := make(chan afterTurn, 3)
	var heldSlots atomic.Int64
	hooks := &OpenAIWSIngressHooks{
		BeforeTurn: func(int) error { heldSlots.Add(1); return nil },
		AfterTurn: func(turn int, result *OpenAIForwardResult, err error) {
			heldSlots.Add(-1)
			after <- afterTurn{turn, result, err}
		},
	}
	errCh := make(chan error, 1)
	warmFrames := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			errCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		_, first, err := conn.Read(r.Context())
		if err != nil {
			errCh <- err
			return
		}
		c, _ := newOpenAIImageGenerationControlTestContext(true, "codex_cli_rs/0.160.0")
		c.Request = r.Clone(r.Context())
		c.Request.Header.Set("User-Agent", "codex_cli_rs/0.160.0")
		errCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), c, conn, account, "test-token", first, hooks)
		warmFrames <- first
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	write := func(payload string) { require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(payload))) }
	read := func() []byte {
		kind, message, readErr := client.Read(ctx)
		require.NoError(t, readErr)
		require.Equal(t, coderws.MessageText, kind)
		return message
	}
	warm := `{"type":"response.create","generate":false,"model":"gpt-6.1-sol","client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"},"input":[{"type":"message","role":"developer","content":"warm-seed-folded-paper-style"}]}`
	write(warm)
	created, acknowledged := read(), read()
	require.Equal(t, "response.created", gjson.GetBytes(created, "type").String())
	require.Equal(t, "response.completed", gjson.GetBytes(acknowledged, "type").String())
	warmID := gjson.GetBytes(acknowledged, "response.id").String()
	require.Equal(t, warmID, gjson.GetBytes(created, "response.id").String())
	prewarmTurn := <-after
	require.NoError(t, prewarmTurn.err)
	require.Nil(t, prewarmTurn.result, "nil is required to skip per-request as well as token billing")
	require.Zero(t, heldSlots.Load(), "prewarm must still release its turn slots")
	require.Empty(t, upstream.requests, "generate=false must not call a model")

	write(fmt.Sprintf(`{"type":"response.create","model":"gpt-6.1-sol","previous_response_id":%q,"client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"},"input":[{"type":"message","role":"user","content":"Generate an image of a folded paper boat."}]}`, warmID))
	generated := read()
	require.Equal(t, "resp_prewarm_business_image", gjson.GetBytes(generated, "response.id").String())
	imageTurn := <-after
	require.NoError(t, imageTurn.err)
	require.NotNil(t, imageTurn.result)
	require.Equal(t, 1, imageTurn.result.ImageCount)
	require.Zero(t, heldSlots.Load())
	require.Len(t, upstream.requests, 1)
	require.Contains(t, string(upstream.bodies[0]), "warm-seed-folded-paper-style")
	require.Contains(t, string(upstream.bodies[0]), "Generate an image of a folded paper boat.")
	require.False(t, gjson.GetBytes(upstream.bodies[0], "generate").Exists())
	require.False(t, gjson.GetBytes(upstream.bodies[0], "previous_response_id").Exists())
	require.Empty(t, upstream.requests[0].Header.Get(responsesLiteHeader))

	write(`{"type":"response.create","model":"gpt-6.1-sol","previous_response_id":"resp_prewarm_business_image","client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"},"input":[{"type":"message","role":"user","content":"Explain the previous result in one sentence."}]}`)
	text := read()
	require.Equal(t, "resp_prewarm_business_text", gjson.GetBytes(text, "response.id").String())
	textTurn := <-after
	require.NoError(t, textTurn.err)
	require.NotNil(t, textTurn.result)
	require.Zero(t, heldSlots.Load())
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "true", upstream.requests[1].Header.Get(responsesLiteHeader))
	require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
	require.NoError(t, <-errCh)
	require.Equal(t, warm, string(<-warmFrames), "client frame remains caller-owned")
}

func TestOpenAIWSHTTPBridgePrewarmWriteFailureReleasesTurnWithoutBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 1
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"must_not_generate\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"))}}
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector()}
	account := &Account{ID: 9902, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Concurrency: 1, Credentials: map[string]any{"access_token": "test-token"}, Extra: map[string]any{"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeHTTPBridge}}
	var heldSlots atomic.Int64
	type completion struct {
		result *OpenAIForwardResult
		err    error
	}
	after := make(chan completion, 1)
	finished := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			finished <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		_, first, err := conn.Read(r.Context())
		if err != nil {
			finished <- err
			return
		}
		c, _ := newOpenAIImageGenerationControlTestContext(true, "codex_cli_rs/0.160.0")
		c.Request = r.Clone(r.Context())
		hooks := &OpenAIWSIngressHooks{
			BeforeTurn: func(int) error {
				heldSlots.Add(1)
				_ = conn.CloseNow() // deterministic downstream ACK failure
				return nil
			},
			AfterTurn: func(_ int, result *OpenAIForwardResult, err error) {
				heldSlots.Add(-1)
				after <- completion{result, err}
			},
		}
		finished <- svc.ProxyResponsesWebSocketFromClient(r.Context(), c, conn, account, "test-token", first, hooks)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","generate":false,"model":"gpt-6.1-sol","input":"warm context"}`)))
	select {
	case done := <-after:
		require.Nil(t, done.result)
		require.Error(t, done.err)
	case <-ctx.Done():
		t.Fatal("prewarm write failure did not release the turn")
	}
	require.Error(t, <-finished)
	require.Zero(t, heldSlots.Load())
	require.Empty(t, upstream.requests)
}
