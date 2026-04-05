package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRelayErrorMessage_SubscriptionQuotaLimit(t *testing.T) {
	rawMessage := "You have reached your subscription quota limit. Please wait for automatic quota refresh in the rolling time window, upgrade to a higher plan, or use a Pay-As-You-Go API Key for unlimited access. (request id: req_123)"

	message, metadata := NormalizeRelayErrorMessage(402, rawMessage, "fallback_req")

	require.Equal(t, "当前套餐额度已用尽，请等待额度自动恢复，或升级套餐后重试。", message)
	require.NotEmpty(t, metadata)

	var parsed RelayErrorDisplayMetadata
	require.NoError(t, Unmarshal(metadata, &parsed))
	require.Equal(t, message, parsed.FriendlyMessage)
	require.Equal(t, "如需立即继续使用，可改用按量付费 API Key。", parsed.ResolutionMessage)
	require.Equal(t, "req_123", parsed.RequestID)
	require.Equal(t, StripRequestID(rawMessage), parsed.RawMessage)
	require.Equal(t, []string{"wait_refresh", "upgrade_plan", "use_payg_key"}, parsed.Actions)
}

func TestNormalizeRelayErrorMessage_NonMatching402PreservesOriginal(t *testing.T) {
	rawMessage := "payment required"

	message, metadata := NormalizeRelayErrorMessage(402, rawMessage, "req_456")

	require.Equal(t, "payment required (request id: req_456)", message)
	require.Nil(t, metadata)
}

func TestNormalizeRelayErrorMessage_Non402PreservesOriginal(t *testing.T) {
	rawMessage := "upstream error"

	message, metadata := NormalizeRelayErrorMessage(500, rawMessage, "req_789")

	require.Equal(t, "upstream error (request id: req_789)", message)
	require.Nil(t, metadata)
}

func TestExtractRequestID(t *testing.T) {
	require.Equal(t, "abc123", ExtractRequestID("bad request (request id: abc123)"))
	require.Empty(t, ExtractRequestID("bad request"))
}
