package common

import (
	"encoding/json"
	"regexp"
	"strings"
)

const (
	subscriptionQuotaFriendlyMessage = "当前套餐额度已用尽，请等待额度自动恢复，或升级套餐后重试。"
	subscriptionQuotaResolution      = "如需立即继续使用，可改用按量付费 API Key。"
)

var requestIDPattern = regexp.MustCompile(`(?i)\(request id:\s*([^)]+)\)`)

type RelayErrorDisplayMetadata struct {
	FriendlyMessage   string   `json:"friendly_message,omitempty"`
	ResolutionMessage string   `json:"resolution_message,omitempty"`
	RequestID         string   `json:"request_id,omitempty"`
	RawMessage        string   `json:"raw_message,omitempty"`
	Actions           []string `json:"actions,omitempty"`
}

func ExtractRequestID(message string) string {
	matches := requestIDPattern.FindStringSubmatch(message)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func StripRequestID(message string) string {
	if message == "" {
		return ""
	}
	return strings.TrimSpace(requestIDPattern.ReplaceAllString(message, ""))
}

func NormalizeRelayErrorMessage(statusCode int, message string, requestID string) (string, json.RawMessage) {
	if friendlyMessage, metadata, ok := normalizeSubscriptionQuotaLimitError(statusCode, message, requestID); ok {
		return friendlyMessage, metadata
	}
	if requestID != "" {
		return MessageWithRequestId(message, requestID), nil
	}
	return message, nil
}

func normalizeSubscriptionQuotaLimitError(statusCode int, message string, requestID string) (string, json.RawMessage, bool) {
	if statusCode != 402 {
		return "", nil, false
	}

	lower := strings.ToLower(message)
	if !strings.Contains(lower, "subscription quota limit") &&
		!strings.Contains(lower, "quota refresh") &&
		!strings.Contains(lower, "pay-as-you-go api key") {
		return "", nil, false
	}

	extractedRequestID := ExtractRequestID(message)
	if extractedRequestID == "" {
		extractedRequestID = requestID
	}

	metadataBytes, err := Marshal(RelayErrorDisplayMetadata{
		FriendlyMessage:   subscriptionQuotaFriendlyMessage,
		ResolutionMessage: subscriptionQuotaResolution,
		RequestID:         extractedRequestID,
		RawMessage:        StripRequestID(message),
		Actions: []string{
			"wait_refresh",
			"upgrade_plan",
			"use_payg_key",
		},
	})
	if err != nil {
		return subscriptionQuotaFriendlyMessage, nil, true
	}

	return subscriptionQuotaFriendlyMessage, metadataBytes, true
}
