package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// This state belongs to one ingress HTTP bridge, never a shared session cache.
// Its immutable input is the bridge's existing successful-response replay.
type codexImageWSContinuation struct {
	input         []json.RawMessage
	inputExists   bool
	responseID    string
	responseCalls []json.RawMessage
	windowID      string
}

type codexImageWSCompatibilityEvaluatedKey struct{}

func codexImageWSCompatibilityEvaluated(ctx context.Context) bool {
	return ctx != nil && ctx.Value(codexImageWSCompatibilityEvaluatedKey{}) == true
}

// A user item, including an empty one, always starts its own intent decision.
// Input strings have the same user semantics as the HTTP Responses input.
func codexImageWSUserItem(item json.RawMessage) bool {
	value := gjson.ParseBytes(item)
	return value.Type == gjson.String ||
		(value.IsObject() && strings.TrimSpace(value.Get("role").String()) == "user")
}

func (state *codexImageWSContinuation) intentSource(payload []byte) ([]byte, bool, error) {
	if state == nil || !state.inputExists || len(state.input) == 0 || isOpenAIWSHTTPBridgePrewarm(payload) {
		return payload, false, nil
	}
	current, _, err := openAIWSExtractNormalizedInputSequence(payload)
	if err != nil {
		return nil, false, err
	}
	for _, item := range current {
		if codexImageWSUserItem(item) {
			return payload, false, nil
		}
	}
	if windowID := openAIWSPayloadCodexWindowID(payload); state.windowID != "" && windowID != "" && state.windowID != windowID {
		return payload, false, nil
	}
	previousID := strings.TrimSpace(gjson.GetBytes(payload, "previous_response_id").String())
	if previousID != "" {
		// An explicit different anchor must not borrow this bridge's history.
		if state.responseID == "" || previousID != state.responseID {
			return payload, false, nil
		}
	} else {
		// Codex may omit previous_response_id on a tool-result delta. All its
		// outputs must match calls from the immediately preceding response.
		var outputs []json.RawMessage
		for _, item := range current {
			if isCodexToolCallOutputItemType(gjson.GetBytes(item, "type").String()) {
				outputs = append(outputs, item)
			}
		}
		if len(outputs) == 0 ||
			!openAIWSRawItemsHaveToolCallContextForOutputs(combineOpenAIWSReplayItems(state.responseCalls, outputs)) {
			return payload, false, nil
		}
	}
	for i := len(state.input) - 1; i >= 0; i-- {
		if !codexImageWSUserItem(state.input[i]) {
			continue
		}
		user := state.input[i]
		if gjson.ParseBytes(user).Type != gjson.String {
			user, err = json.Marshal([]json.RawMessage{user})
			if err != nil {
				return nil, false, err
			}
		}
		// Only the decision sees this latest user. The forwarded delta keeps
		// its original input, tool catalog and history byte-for-byte.
		source, err := sjson.SetRawBytes(payload, "input", user)
		return source, true, err
	}
	return payload, false, nil
}

// OAuth orphan cleanup runs before ingress image preparation. A delta without
// previous_response_id can lose its tool results there, even though this bridge
// holds every matching call. Restore only this verified input to the candidate;
// ingress forwards it only if all image compatibility gates subsequently pass.
func (state *codexImageWSContinuation) restoreMatchedToolOutputInput(payload, original []byte) ([]byte, error) {
	if state == nil || strings.TrimSpace(gjson.GetBytes(original, "previous_response_id").String()) != "" ||
		!openAIWSRawPayloadHasToolCallOutput(original) {
		return payload, nil
	}
	_, hinted, err := state.intentSource(original)
	if err != nil || !hinted {
		return payload, err
	}
	return sjson.SetRawBytes(payload, "input", []byte(gjson.GetBytes(original, "input").Raw))
}

func (s *OpenAIGatewayService) prepareCodexImageWSCompatibility(ctx context.Context, c *gin.Context, account *Account, payload []byte, continuation *codexImageWSContinuation) (context.Context, []byte, bool, error) {
	source, replayHint, err := continuation.intentSource(payload)
	if err != nil {
		return ctx, payload, false, err
	}
	ctx, normalized, promoted, err := s.prepareCodexImageGenerationLiteCompatibility(ctx, c, account, source)
	if replayHint {
		normalized = payload
		if err == nil && promoted {
			normalized, err = removeCodexImageLiteMetadata(payload)
		}
	}
	// The bridge later adds full replay. It must not reinterpret that older
	// input after this decision, particularly when the chain was rejected.
	ctx = context.WithValue(ctx, codexImageWSCompatibilityEvaluatedKey{}, true)
	return ctx, normalized, promoted, err
}
