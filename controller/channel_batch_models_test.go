package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeChannelBatchModels(t *testing.T) {
	models := normalizeChannelBatchModels([]string{
		" gpt-4o, gpt-4.1 ",
		"",
		"gpt-4o",
		"claude-3-5-sonnet",
	})

	require.Equal(t, []string{"gpt-4o", "gpt-4.1", "claude-3-5-sonnet"}, models)
}

func TestMergeChannelBatchModelsAppend(t *testing.T) {
	merged, err := mergeChannelBatchModels(
		[]string{"gpt-4o", "gpt-4.1"},
		[]string{"gpt-4.1", "o4-mini"},
		channelBatchModelModeAppend,
	)

	require.NoError(t, err)
	require.Equal(t, []string{"gpt-4o", "gpt-4.1", "o4-mini"}, merged)
}

func TestMergeChannelBatchModelsReplace(t *testing.T) {
	merged, err := mergeChannelBatchModels(
		[]string{"gpt-4o"},
		[]string{"moonshotai/kimi-k2.6", "kimi-k2.6"},
		channelBatchModelModeReplace,
	)

	require.NoError(t, err)
	require.Equal(t, []string{"moonshotai/kimi-k2.6", "kimi-k2.6"}, merged)
}

func TestMergeChannelBatchModelsRejectsUnknownMode(t *testing.T) {
	_, err := mergeChannelBatchModels([]string{"gpt-4o"}, []string{"o4-mini"}, "bad")

	require.Error(t, err)
}
