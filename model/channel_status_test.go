package model

import (
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func resetChannelStatusTestState(t *testing.T) {
	t.Helper()

	require.NoError(t, DB.AutoMigrate(&Ability{}))
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)

	group2model2channels = nil
	channelsIDM = nil
	channelPollingLocks = sync.Map{}
}

func createChannelWithAbility(t *testing.T, channel *Channel) {
	t.Helper()

	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
}

func marshalOtherInfo(t *testing.T, statusReason string, statusTime int64) string {
	t.Helper()

	payload, err := common.Marshal(map[string]any{
		"status_reason": statusReason,
		"status_time":   statusTime,
	})
	require.NoError(t, err)
	return string(payload)
}

func requireAbilityEnabled(t *testing.T, channelID int, enabled bool) {
	t.Helper()

	var ability Ability
	require.NoError(t, DB.Where("channel_id = ?", channelID).First(&ability).Error)
	require.Equal(t, enabled, ability.Enabled)
}

func requireStatusReason(t *testing.T, channel *Channel, expected string) {
	t.Helper()

	info := channel.GetOtherInfo()
	require.Equal(t, expected, info["status_reason"])
}

func TestCacheUpdateChannelStatusReaddsEnabledChannelToSelection(t *testing.T) {
	resetChannelStatusTestState(t)

	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCache
	})

	lowPriority := int64(1)
	highPriority := int64(10)

	createChannelWithAbility(t, &Channel{
		Id:       1,
		Name:     "low-priority",
		Key:      "sk-low",
		Status:   common.ChannelStatusEnabled,
		Group:    "default",
		Models:   "gpt-5.3-codex",
		Priority: &lowPriority,
	})
	createChannelWithAbility(t, &Channel{
		Id:       2,
		Name:     "high-priority",
		Key:      "sk-high",
		Status:   common.ChannelStatusEnabled,
		Group:    "default",
		Models:   "gpt-5.3-codex",
		Priority: &highPriority,
	})

	InitChannelCache()

	selected, err := GetRandomSatisfiedChannel("default", "gpt-5.3-codex", 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, 2, selected.Id)

	CacheUpdateChannelStatus(2, common.ChannelStatusAutoDisabled)

	selected, err = GetRandomSatisfiedChannel("default", "gpt-5.3-codex", 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, 1, selected.Id)

	CacheUpdateChannelStatus(2, common.ChannelStatusEnabled)

	selected, err = GetRandomSatisfiedChannel("default", "gpt-5.3-codex", 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, 2, selected.Id)
}

func TestUpdateChannelStatusEnableClearsDisableMetadataAndReenablesSelection(t *testing.T) {
	resetChannelStatusTestState(t)

	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCache
	})

	disabledAt := int64(1700000000)
	priority := int64(5)
	channel := &Channel{
		Id:        3,
		Name:      "auto-disabled",
		Key:       "sk-disabled",
		Status:    common.ChannelStatusAutoDisabled,
		Group:     "default",
		Models:    "gpt-5.3-codex",
		Priority:  &priority,
		OtherInfo: marshalOtherInfo(t, "status_code=429", disabledAt),
	}
	createChannelWithAbility(t, channel)

	InitChannelCache()

	ok := UpdateChannelStatus(channel.Id, "", common.ChannelStatusEnabled, "")
	require.True(t, ok)

	reloaded, err := GetChannelById(channel.Id, true)
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusEnabled, reloaded.Status)
	requireStatusReason(t, reloaded, "")

	info := reloaded.GetOtherInfo()
	require.GreaterOrEqual(t, info["status_time"].(float64), float64(disabledAt))
	requireAbilityEnabled(t, channel.Id, true)

	selected, err := GetRandomSatisfiedChannel("default", "gpt-5.3-codex", 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, channel.Id, selected.Id)
}

func TestEnableChannelByTagClearsDisableMetadata(t *testing.T) {
	resetChannelStatusTestState(t)

	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCache
	})

	tag := "codex-tag"
	priority := int64(1)

	createChannelWithAbility(t, &Channel{
		Id:        4,
		Name:      "tagged-one",
		Key:       "sk-tag-1",
		Status:    common.ChannelStatusAutoDisabled,
		Group:     "default",
		Models:    "gpt-5.3-codex",
		Priority:  &priority,
		Tag:       &tag,
		OtherInfo: marshalOtherInfo(t, "status_code=429", 1700000001),
	})
	createChannelWithAbility(t, &Channel{
		Id:        5,
		Name:      "tagged-two",
		Key:       "sk-tag-2",
		Status:    common.ChannelStatusAutoDisabled,
		Group:     "default",
		Models:    "gpt-5.3-codex",
		Priority:  &priority,
		Tag:       &tag,
		OtherInfo: marshalOtherInfo(t, "status_code=429", 1700000002),
	})

	require.NoError(t, EnableChannelByTag(tag))

	for _, channelID := range []int{4, 5} {
		reloaded, err := GetChannelById(channelID, true)
		require.NoError(t, err)
		require.Equal(t, common.ChannelStatusEnabled, reloaded.Status)
		requireStatusReason(t, reloaded, "")
		requireAbilityEnabled(t, channelID, true)
	}
}

func TestEnableChannelByTagClearsDisableMetadataForEnabledChannels(t *testing.T) {
	resetChannelStatusTestState(t)

	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCache
	})

	tag := "enabled-tag"
	priority := int64(1)

	createChannelWithAbility(t, &Channel{
		Id:        6,
		Name:      "already-enabled",
		Key:       "sk-enabled",
		Status:    common.ChannelStatusEnabled,
		Group:     "default",
		Models:    "gpt-5.3-codex",
		Priority:  &priority,
		Tag:       &tag,
		OtherInfo: marshalOtherInfo(t, "status_code=429", 1700000010),
	})

	require.NoError(t, EnableChannelByTag(tag))

	reloaded, err := GetChannelById(6, true)
	require.NoError(t, err)
	require.Equal(t, common.ChannelStatusEnabled, reloaded.Status)
	requireStatusReason(t, reloaded, "")
	requireAbilityEnabled(t, 6, true)
}
