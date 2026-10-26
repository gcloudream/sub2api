package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestCodexImageWSContinuationIntentIsBounded(t *testing.T) {
	state := &codexImageWSContinuation{
		inputExists: true, responseID: "resp_skill", windowID: "window-a",
		input: []json.RawMessage{
			json.RawMessage(`{"role":"user","content":"Generate an image of a folded paper boat."}`),
			json.RawMessage(`{"type":"custom_tool_call","name":"exec","call_id":"call_skill","input":"read skill"}`),
		},
		responseCalls: []json.RawMessage{json.RawMessage(`{"type":"custom_tool_call","name":"exec","call_id":"call_skill","input":"read skill"}`)},
	}
	const output = `{"type":"custom_tool_call_output","call_id":"call_skill","output":"skill content"}`
	for _, tc := range []struct {
		name, fields string
		want         bool
	}{
		{"exact response chain", `"previous_response_id":"resp_skill","input":[` + output + `]`, true},
		{"matching output without response anchor", `"input":[` + output + `]`, true},
		{"empty linked delta", `"previous_response_id":"resp_skill","input":[]`, true},
		{"different explicit response anchor", `"previous_response_id":"resp_other","input":[` + output + `]`, false},
		{"unknown output", `"input":[{"type":"custom_tool_call_output","call_id":"call_other","output":"x"}]`, false},
		{"partially matched outputs", `"input":[` + output + `,{"type":"function_call_output","call_id":"call_other","output":"x"}]`, false},
		{"missing output ID", `"input":[{"type":"custom_tool_call_output","output":"x"}]`, false},
		{"newly supplied call cannot validate its output", `"input":[{"type":"function_call","call_id":"call_forged"},{"type":"function_call_output","call_id":"call_forged","output":"x"}]`, false},
		{"no chain or output", `"input":[]`, false},
		{"new text user overrides image history", `"previous_response_id":"resp_skill","input":[` + output + `,{"role":"user","content":"Explain this result."}]`, false},
		{"new string input overrides image history", `"previous_response_id":"resp_skill","input":"Explain this result."`, false},
		{"empty user still starts a decision", `"previous_response_id":"resp_skill","input":[{"role":"user","content":""}]`, false},
		{"new context window", `"previous_response_id":"resp_skill","client_metadata":{"x-codex-window-id":"window-b"},"input":[` + output + `]`, false},
		{"prewarm cannot inherit", `"generate":false,"previous_response_id":"resp_skill","input":[` + output + `]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := []byte(`{"type":"response.create","model":"gpt-6.1-sol",` + tc.fields + `}`)
			original := append([]byte(nil), payload...)
			source, hinted, err := state.intentSource(payload)
			require.NoError(t, err)
			require.Equal(t, tc.want, hinted)
			require.Equal(t, original, payload)
			if hinted {
				text, _ := codexImageLiteLatestUserText(source)
				require.Equal(t, "Generate an image of a folded paper boat.", text)
			} else {
				require.Equal(t, payload, source)
			}
		})
	}
}

func TestCodexImageWSContinuationRetainsAllCompatibilityGates(t *testing.T) {
	for _, gate := range []string{"allowed", "group denied", "bridge disabled", "strip policy", "passthrough", "Spark", "tool choice none", "other selected tool", "ordinary text turn"} {
		t.Run(gate, func(t *testing.T) {
			svc, c, account := newCodexImageLiteCompatibilityTest(t)
			c.Request.Header.Del(responsesLiteHeader)
			state := &codexImageWSContinuation{inputExists: true, responseID: "resp_skill", input: []json.RawMessage{json.RawMessage(`"Generate an image of a boat."`)}}
			payload := []byte(`{"type":"response.create","model":"gpt-6.1-sol","previous_response_id":"resp_skill","client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true","keep":"value"},"input":[ {"type":"custom_tool_call_output","call_id":"call_skill","output":"read"} ],"tools":[{"type":"custom","name":"exec"}]}`)
			switch gate {
			case "group denied":
				getAPIKeyFromContext(c).Group.AllowImageGeneration = false
			case "bridge disabled":
				svc.cfg.Gateway.CodexImageGenerationBridgeEnabled = false
			case "strip policy":
				account.Extra = map[string]any{"codex_image_generation_explicit_tool_policy": "strip"}
			case "passthrough":
				account.Extra = map[string]any{"openai_passthrough": true}
			case "Spark":
				payload, _ = sjson.SetBytes(payload, "model", "gpt-5.3-codex-spark")
			case "tool choice none":
				payload, _ = sjson.SetBytes(payload, "tool_choice", "none")
			case "other selected tool":
				payload, _ = sjson.SetRawBytes(payload, "tool_choice", []byte(`{"type":"custom","name":"exec"}`))
			case "ordinary text turn":
				state.input = append(state.input, json.RawMessage(`{"role":"user","content":"Explain this result."}`))
			}
			original := append([]byte(nil), payload...)
			ctx, normalized, promoted, err := svc.prepareCodexImageWSCompatibility(context.Background(), c, account, payload, state)
			require.NoError(t, err)
			require.Equal(t, gate == "allowed", promoted)
			require.Equal(t, promoted, codexImageGenerationLiteCompatibilityEnabled(ctx))
			require.True(t, codexImageWSCompatibilityEvaluated(ctx))
			require.Equal(t, original, payload)
			require.Equal(t, gjson.GetBytes(payload, "input").Raw, gjson.GetBytes(normalized, "input").Raw)
			require.Equal(t, gjson.GetBytes(payload, "tools").Raw, gjson.GetBytes(normalized, "tools").Raw)
			require.Equal(t, "value", gjson.GetBytes(normalized, "client_metadata.keep").String())
			require.Equal(t, !promoted, isOpenAIResponsesLiteWebSocketPayload(normalized))
		})
	}
}

func TestCodexImageWSRejectedChainIsNotReconsideredAfterReplay(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_rejected_chain\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n")),
	}}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	c, _ := newOpenAIImageGenerationControlTestContext(true, "codex_cli_rs/0.160.0")
	c.Request.Header.Set(responsesLiteHeader, "true")
	account := &Account{ID: 9904, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}, Extra: map[string]any{"codex_image_generation_bridge": true}}
	state := &codexImageWSContinuation{inputExists: true, responseID: "resp_known", input: []json.RawMessage{json.RawMessage(`{"role":"user","content":"Generate an image of a boat."}`)}}
	delta := []byte(`{"type":"response.create","model":"gpt-6.1-sol","previous_response_id":"resp_unrelated","client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"},"input":[]}`)
	ctx, normalized, promoted, err := svc.prepareCodexImageWSCompatibility(context.Background(), c, account, delta, state)
	require.NoError(t, err)
	require.False(t, promoted)
	// Legacy bridge replay may still contain that older image request. The
	// final request boundary must respect the rejected ingress decision.
	replayed, err := setOpenAIWSPayloadInputSequence(normalized, state.input, true)
	require.NoError(t, err)
	_, err = svc.proxyOpenAIWSHTTPBridgeTurn(ctx, c, account, "test-token", replayed, len(replayed), "gpt-6.1-sol", "", "", "", "", 2, func([]byte) error { return nil })
	require.NoError(t, err)
	require.Equal(t, "true", upstream.lastReq.Header.Get(responsesLiteHeader))
	require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists())
}

func TestCodexImageWSHTTPBridgeSkillGenerationExportFailureRetriesSameImage(t *testing.T) {
	completed := func(id, output string) *http.Response {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf("data: {\"type\":\"response.completed\",\"response\":{\"id\":%q,\"status\":\"completed\",\"model\":\"gpt-6.1-sol\",\"output\":%s,\"usage\":{\"input_tokens\":3,\"output_tokens\":2}}}\n\n", id, output)))}
	}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		completed("resp_skill", `[{"type":"custom_tool_call","name":"exec","call_id":"call_skill","input":"read skill"}]`),
		completed("resp_image_export", `[{"id":"ig_boat","type":"image_generation_call","status":"completed","result":"aW1hZ2U=","size":"1024x1024"},{"type":"custom_tool_call","name":"exec","call_id":"call_export","input":"python export.py"}]`),
		completed("resp_export_retry", `[{"type":"custom_tool_call","name":"exec","call_id":"call_export_retry","input":"python3 export.py"}]`),
		completed("resp_final", `[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Saved image."}]}]`),
		completed("resp_text_tool", `[{"type":"custom_tool_call","name":"exec","call_id":"call_text","input":"read text"}]`),
		completed("resp_text_final", `[]`),
	}}
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	svc := &OpenAIGatewayService{cfg: cfg, httpUpstream: upstream, cache: &stubGatewayCache{}, openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector()}
	account := &Account{ID: 9903, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
		Extra:       map[string]any{"openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeHTTPBridge, "codex_image_generation_bridge": true},
	}
	after := make(chan *OpenAIForwardResult, 6)
	errCh := make(chan error, 1)
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
		c.Request.Header.Set(responsesLiteHeader, "true")
		errCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), c, conn, account, "test-token", first, &OpenAIWSIngressHooks{AfterTurn: func(_ int, result *OpenAIForwardResult, _ error) { after <- result }})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	frames := []string{
		`"input":[{"type":"additional_tools","role":"developer","tools":[{"type":"custom","name":"exec"}]},{"type":"message","role":"user","content":"Generate an image of a folded paper boat."}]`,
		`"previous_response_id":"resp_skill","input":[{"type":"custom_tool_call_output","call_id":"call_skill","output":"skill read"}]`,
		// No response anchor: only the matching call output proves continuation.
		`"input":[{"type":"custom_tool_call_output","call_id":"call_export","output":"Process exited with code 127: python: command not found"}]`,
		`"previous_response_id":"resp_export_retry","input":[{"type":"custom_tool_call_output","call_id":"call_export_retry","output":"/workspace/output/images/boat.png"}]`,
		`"previous_response_id":"resp_final","input":[{"type":"message","role":"user","content":"Explain the result in a sentence."}]`,
		`"previous_response_id":"resp_text_tool","input":[{"type":"custom_tool_call_output","call_id":"call_text","output":"text read"}]`,
	}
	ids := []string{"resp_skill", "resp_image_export", "resp_export_retry", "resp_final", "resp_text_tool", "resp_text_final"}
	generatedImages := 0
	for i, frame := range frames {
		payload := `{"type":"response.create","model":"gpt-6.1-sol","client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true"},` + frame + `}`
		require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(payload)))
		_, message, err := client.Read(ctx)
		require.NoError(t, err)
		require.Equal(t, ids[i], gjson.GetBytes(message, "response.id").String())
		result := <-after
		require.NotNil(t, result)
		generatedImages += result.ImageCount
		if i == 1 {
			require.Equal(t, 1, result.ImageCount)
			require.Len(t, result.wsReplayInput, 2)
			require.Equal(t, "ig_boat", gjson.GetBytes(result.wsReplayInput[0], "id").String())
			require.Equal(t, "call_export", gjson.GetBytes(result.wsReplayInput[1], "call_id").String())
		} else {
			require.Zero(t, result.ImageCount, "replaying an existing input image must not bill another generated image")
		}
		require.Len(t, upstream.requests, i+1)
		body := upstream.bodies[i]
		if i < 4 {
			require.Empty(t, upstream.requests[i].Header.Get(responsesLiteHeader))
			require.True(t, gjson.GetBytes(body, `tools.#(type=="image_generation")`).Exists())
			require.Contains(t, gjson.GetBytes(body, "instructions").String(), "<sub2api-codex-image-delivery>")
		} else {
			require.Equal(t, "true", upstream.requests[i].Header.Get(responsesLiteHeader))
			require.False(t, gjson.GetBytes(body, `tools.#(type=="image_generation")`).Exists())
			require.NotContains(t, gjson.GetBytes(body, "instructions").String(), "<sub2api-codex-image-delivery>")
		}
		if i >= 2 {
			images := gjson.GetBytes(body, `input.#(type=="image_generation_call")#`).Array()
			require.Len(t, images, 1, "export retries keep exactly the original hosted result in the model's input")
			require.JSONEq(t, `{"id":"ig_boat","type":"image_generation_call","status":"completed","result":"aW1hZ2U=","size":"1024x1024"}`, images[0].Raw)
		}
	}
	require.Equal(t, 1, generatedImages, "the mock chain contains one generation followed only by export retries")
	require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
	require.NoError(t, <-errCh)
}
