package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func codexImageDeliveryTestBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var body map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&body))
	return body
}

func codexImageDeliveryTrailingMessage(t *testing.T, body map[string]any) bool {
	t.Helper()
	input, ok := body["input"].([]any)
	if !ok || len(input) == 0 {
		return false
	}
	last, ok := input[len(input)-1].(map[string]any)
	return ok && last["role"] == "developer" && last["type"] == "message" &&
		strings.Contains(extractTextFromContent(last["content"]), codexImageDeliveryMarker)
}

// codexImageDeliveryInputMarkers counts delivery messages in an upstream body.
// The guidance must never enter instructions, which are part of the cached prefix.
func codexImageDeliveryInputMarkers(t *testing.T, body []byte) int {
	t.Helper()
	require.NotContains(t, gjson.GetBytes(body, "instructions").String(), codexImageDeliveryMarker)
	count := 0
	for _, item := range gjson.GetBytes(body, "input").Array() {
		if codexImageDeliveryRawMessage(item) {
			count++
		}
	}
	return count
}

func codexImageDeliveryInputEndsWithMarker(body []byte) bool {
	input := gjson.GetBytes(body, "input").Array()
	return len(input) > 0 && codexImageDeliveryRawMessage(input[len(input)-1])
}

// Encoders may escape "<" in raw JSON, so compare decoded text parts.
func codexImageDeliveryRawMessage(item gjson.Result) bool {
	if item.Get("role").String() != "developer" {
		return false
	}
	for _, part := range item.Get("content").Array() {
		if strings.Contains(part.Get("text").String(), codexImageDeliveryMarker) {
			return true
		}
	}
	return false
}

func TestApplyCodexImageDeliveryInstructions_PreservesRequestAndIsIdempotent(t *testing.T) {
	body := codexImageDeliveryTestBody(t, `{
		"model":"gpt-6.1-sol",
		"instructions":"Keep the user's original rules.  \n",
		"tools":[{"type":"image_generation","output_format":"png"},{"type":"function","name":"exec_command","parameters":{"type":"object"}}],
		"tool_choice":"auto",
		"input":[{"role":"developer","content":"keep developer context"},{"role":"user","content":"Generate an image of a cat."}],
		"client_metadata":{"turn_id":"keep-turn","large_id":9007199254740993}
	}`)
	original := codexImageDeliveryTestBody(t, mustMarshalCodexImageDeliveryTestBody(t, body))
	require.True(t, applyCodexImageDeliveryInstructions(body))
	// Instructions, tools and every earlier input item are the cache prefix and
	// must remain the caller's exact values; only one trailing item is added.
	input := body["input"].([]any)
	originalInput := original["input"].([]any)
	require.Len(t, input, len(originalInput)+1)
	require.Equal(t, originalInput, input[:len(originalInput)])
	require.True(t, codexImageDeliveryTrailingMessage(t, body))
	require.Equal(t, codexImageDeliveryInstructions, extractTextFromContent(input[len(input)-1].(map[string]any)["content"]))
	delete(original, "input")
	others := make(map[string]any, len(body))
	for key, value := range body {
		if key != "input" {
			others[key] = value
		}
	}
	require.Equal(t, original, others)
	after := mustMarshalCodexImageDeliveryTestBody(t, body)
	require.False(t, applyCodexImageDeliveryInstructions(body))
	require.Equal(t, after, mustMarshalCodexImageDeliveryTestBody(t, body))
}

func mustMarshalCodexImageDeliveryTestBody(t *testing.T, body map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	return string(raw)
}

func TestApplyCodexImageDeliveryInstructions_Gates(t *testing.T) {
	const imageUser = `{"role":"user","content":"Generate an image of a cat."}`
	const textUser = `{"type":"message","role":"user","content":[{"type":"input_text","text":"Fix the failing unit test in parser.go."}]}`
	for _, tc := range []struct {
		name, raw string
		want      bool
	}{
		{"image request", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[` + imageUser + `]}`, true},
		{"string image request", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":"画一只橘猫"}`, true},
		{"ordinary coding turn", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[` + textUser + `]}`, false},
		{"ordinary string turn", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":"Explain this stack trace."}`, false},
		{"tool delta without user", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`, false},
		{"continuation after current image call", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"role":"user","content":"来点灵感"},{"type":"image_generation_call","id":"ig_1","status":"completed","result":"aGk="},{"type":"function_call","call_id":"call_1","name":"exec_command","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`, true},
		{"older image before text turn", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[` + imageUser + `,{"type":"image_generation_call","id":"ig_1","status":"completed","result":"aGk="},` + textUser + `]}`, false},
		{"explicit image tool choice", `{"model":"gpt-6.1-sol","tool_choice":{"type":"image_generation"},"tools":[{"type":"image_generation"}],"input":[` + textUser + `]}`, true},
		{"imagegen skill", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"role":"user","content":"$imagegen a lighthouse at dusk"}]}`, true},
		{"attached image edit", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,aGk="},{"type":"input_text","text":"把背景换成白色"}]}]}`, true},
		{"developer and environment items are not the request", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"role":"developer","content":"Generate images when asked."},` + textUser + `]}`, false},
		{"delivery already follows latest user", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[` + imageUser + `,{"type":"message","role":"developer","content":[{"type":"input_text","text":"<sub2api-codex-image-delivery>x"}]}]}`, false},
		{"existing marker in instructions", `{"model":"gpt-6.1-sol","instructions":"existing <sub2api-codex-image-delivery> guidance","tools":[{"type":"image_generation"}],"input":[` + imageUser + `]}`, false},
		{"no tools", `{"model":"gpt-6.1-sol","input":"Generate an image."}`, false},
		{"missing input", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}]}`, false},
		{"passive namespace is not hosted generation", `{"model":"gpt-6.1-sol","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"input":[` + imageUser + `]}`, false},
		{"Spark", `{"model":"gpt-5.3-codex-spark","tools":[{"type":"image_generation"}],"input":[` + imageUser + `]}`, false},
		{"top-level local function", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"function","name":"image_gen.imagegen"}],"input":[` + imageUser + `]}`, false},
		{"legacy local function shape", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"function","function":{"name":"image_gen.imagegen"}}],"input":[` + imageUser + `]}`, false},
		{"namespaced local function", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"input":[` + imageUser + `]}`, false},
		{"explicit namespace on local function", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"function","namespace":"image_gen","name":"imagegen"}],"input":[` + imageUser + `]}`, false},
		{"additional local function", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]},` + imageUser + `]}`, false},
		{"unrelated namespace imagegen name", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"namespace","name":"other","tools":[{"type":"function","name":"imagegen"}]}],"input":[` + imageUser + `]}`, true},
		{"history does not register local tool", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"type":"function_call","name":"image_gen.imagegen","call_id":"past","arguments":"{}"},` + imageUser + `]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := codexImageDeliveryTestBody(t, tc.raw)
			before := mustMarshalCodexImageDeliveryTestBody(t, body)
			require.Equal(t, tc.want, applyCodexImageDeliveryInstructions(body))
			if !tc.want {
				require.Equal(t, before, mustMarshalCodexImageDeliveryTestBody(t, body))
				return
			}
			require.True(t, codexImageDeliveryTrailingMessage(t, body))
			original := codexImageDeliveryTestBody(t, tc.raw)
			require.Equal(t, original["instructions"], body["instructions"])
			require.Equal(t, original["tool_choice"], body["tool_choice"])
		})
	}
	require.False(t, applyCodexImageDeliveryInstructions(nil))
	require.False(t, applyCodexImageDeliveryInstructions(map[string]any{}))
}

func TestCodexImageDeliveryRequestHint(t *testing.T) {
	for _, text := range []string{
		"请生成一张窗边橘猫的图片",
		"画一只在月球上的柯基",
		"帮我设计一个咖啡店的 logo",
		"来张赛博朋克城市夜景",
		"做一张春节海报，红色主题",
		"出图：极简风格的山水",
		"Generate an image of a folded paper boat.",
		"Please draw a fox reading a book",
		"create a poster for our hackathon",
		"$imagegen 一座灯塔",
	} {
		require.True(t, codexImageDeliveryRequestHint(text, false), text)
	}
	require.True(t, codexImageDeliveryRequestHint("把背景去掉", true))
	for _, text := range []string{
		"",
		"请解释这个函数为什么返回空值",
		"修复 parser.go 里失败的单元测试",
		"pnpm build 之后报错了，帮我看看",
		"Fix the failing unit test in parser.go.",
		"Refactor the image loader to stream bytes",
		"How does the image_generation tool work?",
		"Explain the result in a sentence.",
	} {
		require.False(t, codexImageDeliveryRequestHint(text, false), text)
	}
}

func TestApplyCodexImageDeliveryInstructions_ReviewedDeliveryContract(t *testing.T) {
	// These are the delivery boundaries, not promises that a client executor
	// bypasses its permission policy or that model output has already saved a file.
	for _, rule := range []string{
		"only when this turn actually generates a new image",
		"never CODEX_SESSION_ID",
		"verify its session_meta payload.id equals that UUID",
		"CURRENT turn",
		"If the current turn cannot be identified, stop",
		"at most 20 seconds",
		"Do not delegate the export",
		"first check for an existing python3 executable",
		"do not assume a python alias exists",
		"retry only the export of the same completed image_generation_call ID and result",
		"Never call image_generation again to repair an export or file operation",
		"ASCII letters, digits, underscore and hyphen only",
		"do not install software or bypass the client's existing filesystem permissions",
		"image_markdown, and download_markdown",
		"same complete ABSOLUTE local path",
		"copy these verified image_markdown and download_markdown strings exactly",
		"never claim a nonexistent preview or regenerate solely to repair delivery",
	} {
		require.Contains(t, codexImageDeliveryText, rule)
	}
}
