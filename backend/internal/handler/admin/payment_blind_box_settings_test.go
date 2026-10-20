//go:build unit

package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasPaymentFieldsRecognizesExplicitBlindBoxDisable(t *testing.T) {
	var req UpdateSettingsRequest
	require.NoError(t, json.Unmarshal([]byte(`{"payment_blind_box_enabled":false}`), &req))
	require.NotNil(t, req.PaymentBlindBoxEnabled)
	require.False(t, *req.PaymentBlindBoxEnabled)
	require.True(t, hasPaymentFields(req))
	require.False(t, hasPaymentFields(UpdateSettingsRequest{}))
}
