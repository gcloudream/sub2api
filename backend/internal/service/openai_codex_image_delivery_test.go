package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func codexImageDeliveryTestBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	var body map[string]any
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&body))
	return body
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
	before, err := json.Marshal(body)
	require.NoError(t, err)
	originalInstructions := body["instructions"].(string)
	require.True(t, applyCodexImageDeliveryInstructions(body))
	got := body["instructions"].(string)
	require.True(t, strings.HasPrefix(got, originalInstructions+"\n\n"), "existing whitespace and rules must remain intact")
	require.Contains(t, got, codexImageDeliveryInstructions)
	require.Equal(t, 1, strings.Count(got, codexImageDeliveryMarker))
	// Only instructions may change; namespaces, history, choices and large IDs
	// must remain the caller's original values.
	otherFields := codexImageDeliveryTestBody(t, string(before))
	delete(otherFields, "instructions")
	actualOtherFields := make(map[string]any, len(body))
	for key, value := range body {
		if key != "instructions" {
			actualOtherFields[key] = value
		}
	}
	require.Equal(t, otherFields, actualOtherFields)
	require.False(t, applyCodexImageDeliveryInstructions(body))
	require.Equal(t, got, body["instructions"])
}

func TestApplyCodexImageDeliveryInstructions_Gates(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      bool
	}{
		{"hosted tool without instructions", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}]}`, true},
		{"null instructions", `{"model":"gpt-6.1-sol","instructions":null,"tools":[{"type":"image_generation"}]}`, true},
		{"generation bridge marker does not suppress delivery", `{"model":"gpt-6.1-sol","instructions":"<sub2api-codex-image-generation>existing bridge rules</sub2api-codex-image-generation>","tools":[{"type":"image_generation"}]}`, true},
		{"explicit none remains untouched", `{"model":"gpt-6.1-sol","tool_choice":"none","tools":[{"type":"image_generation"}]}`, true},
		{"no tools", `{"model":"gpt-6.1-sol","input":"Generate an image."}`, false},
		{"passive namespace is not hosted generation", `{"model":"gpt-6.1-sol","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]}`, false},
		{"additional hosted declaration is not top level", `{"model":"gpt-6.1-sol","input":[{"type":"additional_tools","tools":[{"type":"image_generation"}]}]}`, false},
		{"Spark", `{"model":"gpt-5.3-codex-spark","tools":[{"type":"image_generation"}]}`, false},
		{"invalid instructions remain for validation", `{"model":"gpt-6.1-sol","instructions":{"keep":"invalid"},"tools":[{"type":"image_generation"}]}`, false},
		{"existing delivery marker", `{"model":"gpt-6.1-sol","instructions":"existing <sub2api-codex-image-delivery> guidance","tools":[{"type":"image_generation"}]}`, false},
		{"top-level local function", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"function","name":"image_gen.imagegen"}]}`, false},
		{"legacy local function shape", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"function","function":{"name":"image_gen.imagegen"}}]}`, false},
		{"namespaced local function", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]}`, false},
		{"explicit namespace on local function", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"function","namespace":"image_gen","name":"imagegen"}]}`, false},
		{"additional local function", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]}]}`, false},
		{"unrelated namespace imagegen name", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"},{"type":"namespace","name":"other","tools":[{"type":"function","name":"imagegen"}]}]}`, true},
		{"history does not register local tool", `{"model":"gpt-6.1-sol","tools":[{"type":"image_generation"}],"input":[{"type":"function_call","name":"image_gen.imagegen","call_id":"past","arguments":"{}"}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := codexImageDeliveryTestBody(t, tc.raw)
			before, err := json.Marshal(body)
			require.NoError(t, err)
			require.Equal(t, tc.want, applyCodexImageDeliveryInstructions(body))
			if !tc.want {
				after, marshalErr := json.Marshal(body)
				require.NoError(t, marshalErr)
				require.Equal(t, string(before), string(after))
			} else {
				require.Contains(t, body["instructions"], codexImageDeliveryMarker)
				original := codexImageDeliveryTestBody(t, tc.raw)
				require.Equal(t, original["tool_choice"], body["tool_choice"])
				if instructions, ok := original["instructions"].(string); ok && instructions != "" {
					require.True(t, strings.HasPrefix(body["instructions"].(string), instructions+"\n\n"))
				}
			}
		})
	}
	require.False(t, applyCodexImageDeliveryInstructions(nil))
	require.False(t, applyCodexImageDeliveryInstructions(map[string]any{}))
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
