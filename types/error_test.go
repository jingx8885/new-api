package types

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestNewAPIErrorToOpenAIErrorIncludesMetadata(t *testing.T) {
	metadata, err := common.Marshal(map[string]any{
		"friendly_message":   "当前套餐额度已用尽，请等待额度自动恢复，或升级套餐后重试。",
		"resolution_message": "如需立即继续使用，可改用按量付费 API Key。",
		"request_id":         "req_123",
	})
	require.NoError(t, err)

	newAPIError := NewErrorWithStatusCode(errors.New("quota exceeded"), ErrorCodeBadResponseStatusCode, 402)
	newAPIError.Metadata = metadata

	openAIError := newAPIError.ToOpenAIError()

	require.Equal(t, "quota exceeded", openAIError.Message)
	require.JSONEq(t, string(metadata), string(openAIError.Metadata))
}
