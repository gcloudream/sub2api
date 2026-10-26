package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIWSHTTPBridgeReplayPreservesCompletedHostedImage(t *testing.T) {
	collector := &openAIWSToolCallReplayCollector{}
	collector.AddEvent("response.output_item.done", []byte(`{"item":{"id":"ig_saved","type":"image_generation_call","status":"in_progress"}}`))
	require.Empty(t, collector.HTTPBridgeItems(), "an incomplete image is not a usable replay result")
	const image = `{"id":"ig_saved","type":"image_generation_call","status":"completed","result":"aW1hZ2U=","revised_prompt":"a folded boat","output_format":"png","size":"1024x1024"}`
	terminal := []byte(`{"response":{"output":[` + image + `,{"id":"fc_export","type":"custom_tool_call","name":"exec","call_id":"call_export","input":"export already-generated image"},{"id":"message_skip","type":"message","role":"assistant","content":[]},{"id":"ig_failed","type":"image_generation_call","status":"failed","result":""},{"id":"ig_empty","type":"image_generation_call","status":"completed","result":""},{"id":"ig_pending","type":"image_generation_call","status":"in_progress","result":"partial"}]}}`)
	collector.AddEvent("response.completed", terminal)
	collector.AddEvent("response.done", terminal)
	items := collector.HTTPBridgeItems()
	require.Len(t, items, 2, "the output_item and terminal duplicate must not repeat the hosted image")
	require.Equal(t, image, string(items[0]), "the native Responses image shape and result bytes must stay intact")
	require.Equal(t, "call_export", gjson.GetBytes(items[1], "call_id").String())
	require.Len(t, collector.Items(), 1, "native upstream WS keeps its existing tool-only replay contract")
	items[0] = nil
	require.Equal(t, image, string(collector.HTTPBridgeItems()[0]), "returned slice headers are independent")
}

func TestOpenAIWSHTTPBridgeImageCompletionCannotReplaceAnotherItemType(t *testing.T) {
	collector := &openAIWSToolCallReplayCollector{}
	const call = `{"id":"collision","type":"custom_tool_call","name":"exec","call_id":"call_export","input":"export image"}`
	collector.AddEvent("response.output_item.done", []byte(`{"item":`+call+`}`))
	collector.AddEvent("response.completed", []byte(`{"response":{"output":[{"id":"collision","type":"image_generation_call","status":"completed","result":"aW1hZ2U="}]}}`))
	items := collector.HTTPBridgeItems()
	require.Len(t, items, 1)
	require.Equal(t, call, string(items[0]), "an invalid ID collision must not replace a retained tool call")
}
