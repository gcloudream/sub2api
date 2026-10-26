package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func newCodexImageLiteCompatibilityTest(t *testing.T) (*OpenAIGatewayService, *gin.Context, *Account) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header.Set("User-Agent", "Codex Desktop/0.160.0")
	c.Request.Header.Set(responsesLiteHeader, "true")
	groupID := int64(6)
	c.Set("api_key", &APIKey{GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformOpenAI, AllowImageGeneration: true}})
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.CodexImageGenerationBridgeEnabled = true
	account := &Account{ID: 33, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	return svc, c, account
}

func codexImageLiteUserBody(t *testing.T, text string) []byte {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"model": "gpt-6.1-sol",
		"input": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": text}}}},
	})
	require.NoError(t, err)
	return encoded
}

func TestCodexImageLiteCompatibility_DirectRequestsOnly(t *testing.T) {
	cases := []struct {
		name, text string
		image      bool
		want       bool
	}{
		{"Chinese generation", "请直接生成一张小猫在窗边的图片。", false, true},
		{"Chinese drawing without image noun", "帮我画一只戴帽子的猫。", false, true},
		{"Chinese explicit edit", "把这张照片的背景换成蓝色。", true, true},
		{"Chinese attached edit", "把背景换成蓝色。", true, true},
		{"English generation", "Please generate an image of a cat beside a window.", false, true},
		{"English drawing", "Draw a cat sleeping in the sun.", false, true},
		{"English edit", "Please edit this image to make the sky blue.", true, true},
		{"English attached edit", "Remove the background.", true, true},
		{"Explicit request with code refusal", "请生成一张小猫图片，不要写代码。", false, true},
		{"English request with code refusal", "Generate an image of a cat; do not write code.", false, true},
		{"Chinese request forbids separate image API", "请直接生成一张猫咪图片。不要调用独立Images API，不要使用脚本。", false, true},
		{"Code refusal in a separate sentence", "请生成一张猫咪图片。请使用请求提供的原生工具。不要编写脚本。", false, true},
		{"Code refusal in a separate clause", "请生成一张猫咪图片，使用请求提供的原生工具，不要编写脚本。", false, true},
		{"English request forbids separate image API", "Generate an image of a cat. Do not use scripts or call the separate Images API.", false, true},
		{"Content constraint is not a cancelled request", "画一张风景画，不要画人物。", false, true},
		{"Polite English request", "Can you please generate an image of a cat?", false, true},
		{"Skill tag before direct request", "$imagegen\n请生成一张小猫图片。", false, true},
		{"Upload button icon is a requested image", "请生成一个上传按钮图标。", false, true},
		{"Real acceptance prompt", "请生成且仅生成一张图片：民国时期上海安静的街巷，一位身穿素雅旗袍的成年女子缓步走过石板路，复古卷发，午后柔光，写实电影质感，无文字、无水印。请直接使用当前请求提供的原生图片生成工具。不要编写或运行脚本，不要调用独立 Images API，不要输出提示词代替图片。生成完成后简短确认，不要再次生成。", false, true},
		{"Ordinary code task", "修复登录页面的按钮。", false, false},
		{"Create branch", "创建一个分支。", false, false},
		{"Generate user table", "生成一张用户表。", false, false},
		{"Create object without image noun", "请创建一个用户。", false, false},
		{"Chinese capability", "你可以生成图片吗？", false, false},
		{"English capability", "Can you generate images?", false, false},
		{"Capability analysis", "分析一下幻想家能否生成图片。", false, false},
		{"Polite capability analysis", "帮我分析一下能不能生成图片。", false, false},
		{"Polite capability question", "请问你能不能生成图片？", false, false},
		{"Chinese negative", "不要生成图片，只解释步骤。", false, false},
		{"English negative", "Do not generate an image of a cat.", false, false},
		{"Chinese script authoring", "帮我写一个生成图片的脚本。", false, false},
		{"Chinese website authoring", "帮我写一个生成图片的网站。", false, false},
		{"English script authoring", "Write a Python script to generate an image of a cat.", false, false},
		{"Create script", "Create a Python script to generate an image.", false, false},
		{"Explain image API", "Explain how to generate an image through the API.", false, false},
		{"Prompt authoring", "请帮我写一份生成图片的提示词。", false, false},
		{"Generate prompt", "生成一份图像提示词。", false, false},
		{"Quoted code example", "Example:\n```\ngenerate an image\n```", false, false},
		{"No attached image for implicit edit", "Remove the background.", false, false},
		{"Image understanding", "Describe this image.", true, false},
		{"Translation is not generation", "翻译这句话：Generate an image of a cat.", false, false},
		{"Print quoted sentence", "Print the sentence \"Generate an image of a cat\".", false, false},
		{"Generate text about an image", "Generate a paragraph about an image of a cat.", false, false},
		{"Generate Chinese text about an image", "生成一段关于图片的说明文字", false, false},
		{"Discussion of an earlier request", "我昨天让你画一只猫，今天想知道用了哪个模型", false, false},
		{"Explain quoted sentence", "这句英文是什么意思：Generate an image of a cat.", false, false},
		{"Edit without attached image", "Please edit this image to make the sky blue.", false, false},
		{"Image component code", "修改图片上传组件的逻辑", true, false},
		{"Image alt property code", "修改图片的 alt 属性", true, false},
		{"Image metadata code", "Modify the image metadata in this JSON", true, false},
		{"Create image upload component", "创建图片上传组件", false, false},
		{"English create image upload component", "Create an image upload component", false, false},
		{"Empty", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, codexImageLiteHasDirectImageRequest(tc.text, tc.image))
		})
	}
}

func TestCodexImageLiteCompatibility_Gates(t *testing.T) {
	cases := []struct {
		name   string
		change func(*OpenAIGatewayService, *gin.Context, *Account, []byte) []byte
		want   bool
	}{
		{name: "enabled", want: true},
		{name: "setup token", change: func(_ *OpenAIGatewayService, _ *gin.Context, a *Account, b []byte) []byte {
			a.Type = AccountTypeSetupToken
			return b
		}, want: true},
		{name: "not Codex", change: func(_ *OpenAIGatewayService, c *gin.Context, _ *Account, b []byte) []byte {
			c.Request.Header.Set("User-Agent", "other-client")
			return b
		}},
		{name: "originator identifies Codex", change: func(_ *OpenAIGatewayService, c *gin.Context, _ *Account, b []byte) []byte {
			c.Request.Header.Set("User-Agent", "other-client")
			c.Request.Header.Set("originator", "Codex Desktop")
			return b
		}, want: true},
		{name: "force Codex", change: func(s *OpenAIGatewayService, c *gin.Context, _ *Account, b []byte) []byte {
			c.Request.Header.Set("User-Agent", "other-client")
			s.cfg.Gateway.ForceCodexCLI = true
			return b
		}, want: true},
		{name: "bridge off", change: func(s *OpenAIGatewayService, _ *gin.Context, _ *Account, b []byte) []byte {
			s.cfg.Gateway.CodexImageGenerationBridgeEnabled = false
			return b
		}},
		{name: "account bridge disabled", change: func(_ *OpenAIGatewayService, _ *gin.Context, a *Account, b []byte) []byte {
			a.Extra = map[string]any{featureKeyCodexImageGenerationBridge: false}
			return b
		}},
		{name: "group disallows images", change: func(_ *OpenAIGatewayService, c *gin.Context, _ *Account, b []byte) []byte {
			getAPIKeyFromContext(c).Group.AllowImageGeneration = false
			return b
		}},
		{name: "API key account excluded", change: func(_ *OpenAIGatewayService, _ *gin.Context, a *Account, b []byte) []byte {
			a.Type = AccountTypeAPIKey
			return b
		}},
		{name: "other platform excluded", change: func(_ *OpenAIGatewayService, _ *gin.Context, a *Account, b []byte) []byte {
			a.Platform = PlatformGrok
			return b
		}},
		{name: "passthrough excluded", change: func(_ *OpenAIGatewayService, _ *gin.Context, a *Account, b []byte) []byte {
			a.Extra = map[string]any{"openai_passthrough": true}
			return b
		}},
		{name: "legacy passthrough excluded", change: func(_ *OpenAIGatewayService, _ *gin.Context, a *Account, b []byte) []byte {
			a.Extra = map[string]any{"openai_oauth_passthrough": true}
			return b
		}},
		{name: "strip policy", change: func(_ *OpenAIGatewayService, _ *gin.Context, a *Account, b []byte) []byte {
			a.Extra = map[string]any{featureKeyCodexImageGenerationExplicitToolPolicy: "strip"}
			return b
		}},
		{name: "compact", change: func(_ *OpenAIGatewayService, c *gin.Context, _ *Account, b []byte) []byte {
			c.Request.URL.Path = "/v1/responses/compact"
			return b
		}},
		{name: "Spark", change: func(_ *OpenAIGatewayService, _ *gin.Context, _ *Account, b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("gpt-6.1-sol"), []byte("gpt-5.3-codex-spark"))
		}},
		{name: "mapped Spark", change: func(_ *OpenAIGatewayService, _ *gin.Context, a *Account, b []byte) []byte {
			a.Credentials = map[string]any{"model_mapping": map[string]any{"gpt-6.1-sol": "gpt-5.3-codex-spark"}}
			return b
		}},
		{name: "not Lite", change: func(_ *OpenAIGatewayService, c *gin.Context, _ *Account, b []byte) []byte {
			c.Request.Header.Del(responsesLiteHeader)
			return b
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, c, a := newCodexImageLiteCompatibilityTest(t)
			body := codexImageLiteUserBody(t, "请生成一张猫咪图片。")
			if tc.change != nil {
				body = tc.change(s, c, a, body)
			}
			original := append([]byte(nil), body...)
			header := c.Request.Header.Clone()
			ctx, got, enabled, err := s.prepareCodexImageGenerationLiteCompatibility(context.Background(), c, a, body)
			require.NoError(t, err)
			require.Equal(t, tc.want, enabled)
			require.Equal(t, tc.want, codexImageGenerationLiteCompatibilityEnabled(ctx))
			require.Equal(t, original, body)
			require.Equal(t, original, got)
			require.Equal(t, header, c.Request.Header)
		})
	}
}

func TestCodexImageLiteCompatibility_ToolChoiceIsRespected(t *testing.T) {
	for _, tc := range []struct {
		choice string
		want   bool
	}{
		{`"auto"`, true}, {`"required"`, true}, {`{"type":"image_generation"}`, true},
		{`"none"`, false}, {`{"type":"function","name":"search"}`, false},
		{`{"type":"allowed_tools","tools":[{"type":"function","name":"search"}]}`, false},
	} {
		t.Run(tc.choice, func(t *testing.T) {
			s, c, a := newCodexImageLiteCompatibilityTest(t)
			body := []byte(`{"model":"gpt-6.1-sol","tool_choice":` + tc.choice + `,"input":"Generate an image of a cat."}`)
			_, got, enabled, err := s.prepareCodexImageGenerationLiteCompatibility(context.Background(), c, a, body)
			require.NoError(t, err)
			require.Equal(t, tc.want, enabled)
			require.Equal(t, string(body), string(got))
		})
	}
}

func TestCodexImageLiteCompatibility_ExplicitHostedIntent(t *testing.T) {
	for _, tc := range []struct {
		name, declarations string
		want               bool
	}{
		{"native hosted tool", `"tools":[{"type":"image_generation"}],"input":"A cat"`, true},
		{"explicit image tool choice", `"tool_choice":{"type":"image_generation"},"input":"A cat"`, true},
		{"native tool with none choice", `"tools":[{"type":"image_generation"}],"tool_choice":"none","input":"A cat"`, false},
		{"native tool with another selected tool", `"tools":[{"type":"image_generation"}],"tool_choice":{"type":"function","name":"search"},"input":"A cat"`, false},
		{"passive top-level namespace", `"tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"input":"A cat"`, false},
		{"passive additional namespace", `"input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]},{"role":"user","content":"A cat"}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, c, a := newCodexImageLiteCompatibilityTest(t)
			body := []byte(`{"model":"gpt-6.1-sol",` + tc.declarations + `}`)
			ctx, got, enabled, err := s.prepareCodexImageGenerationLiteCompatibility(context.Background(), c, a, body)
			require.NoError(t, err)
			require.Equal(t, tc.want, enabled)
			require.Equal(t, tc.want, codexImageGenerationLiteCompatibilityEnabled(ctx))
			require.Equal(t, body, got)
		})
	}
}

func TestCodexImageLiteCompatibility_OnlyLatestUserText(t *testing.T) {
	s, c, a := newCodexImageLiteCompatibilityTest(t)
	for _, input := range []string{
		`[{"role":"developer","content":"Generate an image of a cat."},{"role":"user","content":"Fix the login button."}]`,
		`[{"role":"user","content":"Generate an image of a cat."},{"role":"assistant","content":"Done"},{"role":"user","content":"Explain the login logic."}]`,
		`[{"type":"additional_tools","tools":[{"type":"function","name":"image","description":"Generate an image of a cat."}]},{"role":"user","content":"Fix a typo."}]`,
		`[{"role":"user","content":[{"type":"input_image","image_url":"https://example.test/generate-an-image-of-a-cat.png"}]}]`,
	} {
		body := []byte(`{"model":"gpt-6.1-sol","instructions":"Generate an image of a cat.","input":` + input + `}`)
		_, _, enabled, err := s.prepareCodexImageGenerationLiteCompatibility(context.Background(), c, a, body)
		require.NoError(t, err)
		require.False(t, enabled, input)
	}
	// Tool outputs and assistant progress after the latest user do not erase it.
	body := []byte(`{"model":"gpt-6.1-sol","input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,eA=="},{"type":"input_text","text":"Remove the background."}]},{"type":"function_call_output","output":"ready"}]}`)
	_, _, enabled, err := s.prepareCodexImageGenerationLiteCompatibility(context.Background(), c, a, body)
	require.NoError(t, err)
	require.True(t, enabled)
}

func TestCodexImageLiteCompatibility_WSMetadataAndJSONRemainIntact(t *testing.T) {
	s, c, a := newCodexImageLiteCompatibilityTest(t)
	c.Request.Header.Del(responsesLiteHeader)
	body := []byte(`{"type":"response.create","model":"gpt-6.1-sol","client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true","turn":"keep"},"large_id":9007199254740993,"input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"functions","tools":[{"type":"custom","name":"exec","format":{"type":"grammar","definition":"start: /[\\s\\S]+/"}}]}]},{"role":"developer","content":"keep instructions"},{"role":"user","content":"Please generate an image of a cat."},{"type":"custom_tool_call","namespace":"functions","name":"exec","call_id":"fc_keep","input":"keep()"}]}`)
	original := append([]byte(nil), body...)
	ctx, got, enabled, err := s.prepareCodexImageGenerationLiteCompatibility(context.Background(), c, a, body)
	require.NoError(t, err)
	require.True(t, enabled)
	require.True(t, codexImageGenerationLiteCompatibilityEnabled(ctx))
	require.Equal(t, original, body)
	require.False(t, isOpenAIResponsesLiteWebSocketPayload(got))
	require.Equal(t, "9007199254740993", gjson.GetBytes(got, "large_id").Raw)
	require.Equal(t, gjson.GetBytes(body, "input").Raw, gjson.GetBytes(got, "input").Raw)
	require.Equal(t, "keep", gjson.GetBytes(got, "client_metadata.turn").String())
	require.Empty(t, c.Request.Header.Get(responsesLiteHeader))
	// Re-evaluate policy on failover rather than letting the old flag authorize
	// an account with the bridge disabled.
	a.Extra = map[string]any{featureKeyCodexImageGenerationBridge: false}
	ctx, unchanged, enabled, err := s.prepareCodexImageGenerationLiteCompatibility(ctx, c, a, original)
	require.NoError(t, err)
	require.False(t, enabled)
	require.False(t, codexImageGenerationLiteCompatibilityEnabled(ctx))
	require.Equal(t, original, unchanged)
}

func TestCodexImageLiteCompatibility_RequestBoundary(t *testing.T) {
	body := `{"model":"gpt-6.1-sol","client_metadata":{"ws_request_header_x_openai_internal_codex_responses_lite":"true","keep":9007199254740993},"input":"Generate an image of a cat."}`
	ctx := context.WithValue(context.Background(), codexImageGenerationLiteCompatibilityContextKey{}, true)
	req := httptest.NewRequest(http.MethodPost, "https://example.test/responses", strings.NewReader(body)).WithContext(ctx)
	// httptest.NewRequest does not install the standard client GetBody callback.
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(body)), nil }
	req.Header.Set(responsesLiteHeader, "true")
	req.Header.Set("X-Keep", "yes")
	require.NoError(t, applyCodexImageGenerationLiteCompatibility(req))
	got, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.False(t, isOpenAIResponsesLiteWebSocketPayload(got))
	require.Equal(t, "9007199254740993", gjson.GetBytes(got, "client_metadata.keep").Raw)
	require.Empty(t, req.Header.Get(responsesLiteHeader))
	require.Equal(t, "yes", req.Header.Get("X-Keep"))
	require.Equal(t, int64(len(got)), req.ContentLength)
	for i := 0; i < 2; i++ {
		replay, replayErr := req.GetBody()
		require.NoError(t, replayErr)
		replayed, readErr := io.ReadAll(replay)
		require.NoError(t, readErr)
		require.NoError(t, replay.Close())
		require.Equal(t, got, replayed)
	}
	// This operation is limited to marked requests.
	unmarked, err := http.NewRequest(http.MethodPost, "https://example.test/responses", strings.NewReader(body))
	require.NoError(t, err)
	unmarked.Header.Set(responsesLiteHeader, "true")
	require.NoError(t, applyCodexImageGenerationLiteCompatibility(unmarked))
	require.Equal(t, "true", unmarked.Header.Get(responsesLiteHeader))
	untouched, err := io.ReadAll(unmarked.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(untouched))
}

func TestCodexImageLiteCompatibility_RequestBoundaryFailureDoesNotMutate(t *testing.T) {
	ctx := context.WithValue(context.Background(), codexImageGenerationLiteCompatibilityContextKey{}, true)
	for _, tc := range []struct {
		name    string
		getBody func() (io.ReadCloser, error)
	}{
		{"non replayable", nil},
		{"replay error", func() (io.ReadCloser, error) { return nil, errors.New("cannot replay") }},
		{"invalid JSON", func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("not json")), nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "https://example.test/responses", strings.NewReader("original-stream")).WithContext(ctx)
			req.GetBody = tc.getBody
			req.Header.Set(responsesLiteHeader, "true")
			require.Error(t, applyCodexImageGenerationLiteCompatibility(req))
			require.Equal(t, "true", req.Header.Get(responsesLiteHeader))
			got, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Equal(t, "original-stream", string(got))
		})
	}
}
