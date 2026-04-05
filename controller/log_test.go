package controller

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type batchConsumeResponseData struct {
	Total     int64                        `json:"total"`
	Items     []model.Log                  `json:"items"`
	Summary   model.BatchConsumeSummary    `json:"summary"`
	UserStats []model.BatchConsumeUserStat `json:"user_stats"`
}

func setupLogControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := setupTokenControllerTestDB(t)
	if err := db.AutoMigrate(&model.Log{}, &model.Channel{}); err != nil {
		t.Fatalf("failed to migrate log tables: %v", err)
	}
	return db
}

func seedConsumeLog(t *testing.T, db *gorm.DB, log model.Log) {
	t.Helper()

	if err := db.Create(&log).Error; err != nil {
		t.Fatalf("failed to create consume log: %v", err)
	}
}

func TestGetBatchConsumeLogsReturnsSummaryAndPagedItems(t *testing.T) {
	db := setupLogControllerTestDB(t)
	if err := db.Create(&model.Channel{Id: 11, Name: "batch-channel"}).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	seedConsumeLog(t, db, model.Log{
		UserId:           1,
		Username:         "alice",
		CreatedAt:        1710000001,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            120,
		PromptTokens:     10,
		CompletionTokens: 20,
		ChannelId:        11,
	})
	seedConsumeLog(t, db, model.Log{
		UserId:           2,
		Username:         "bob",
		CreatedAt:        1710000002,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o",
		Quota:            200,
		PromptTokens:     15,
		CompletionTokens: 25,
		ChannelId:        11,
	})
	seedConsumeLog(t, db, model.Log{
		UserId:           1,
		Username:         "alice",
		CreatedAt:        1710000003,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o-mini",
		Quota:            80,
		PromptTokens:     5,
		CompletionTokens: 8,
		ChannelId:        11,
	})
	seedConsumeLog(t, db, model.Log{
		UserId:    3,
		Username:  "ignored",
		CreatedAt: 1710000004,
		Type:      model.LogTypeTopup,
		ModelName: "gpt-4o",
		Quota:     999,
	})

	body := map[string]any{
		"usernames":       []string{" alice ", "bob", "alice", ""},
		"start_timestamp": int64(1710000000),
		"end_timestamp":   int64(1710000010),
		"page":            1,
		"page_size":       2,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/log/batch/consume", body, 1)

	GetBatchConsumeLogs(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var data batchConsumeResponseData
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode batch consume response: %v", err)
	}

	if data.Total != 3 {
		t.Fatalf("expected total 3, got %d", data.Total)
	}
	if len(data.Items) != 2 {
		t.Fatalf("expected 2 paged items, got %d", len(data.Items))
	}
	if data.Items[0].Username != "alice" || data.Items[0].CreatedAt != 1710000003 {
		t.Fatalf("expected latest item to be alice latest log, got %+v", data.Items[0])
	}
	if data.Items[0].ChannelName != "batch-channel" {
		t.Fatalf("expected channel name to be enriched, got %q", data.Items[0].ChannelName)
	}
	if data.Summary.Quota != 400 {
		t.Fatalf("expected summary quota 400, got %d", data.Summary.Quota)
	}
	if data.Summary.ConsumeCount != 3 {
		t.Fatalf("expected summary consume count 3, got %d", data.Summary.ConsumeCount)
	}
	if len(data.UserStats) != 2 {
		t.Fatalf("expected 2 user stats, got %d", len(data.UserStats))
	}
	if data.UserStats[0].Username != "alice" || data.UserStats[0].Quota != 200 || data.UserStats[0].ConsumeCount != 2 || data.UserStats[0].LastCreatedAt != 1710000003 {
		t.Fatalf("unexpected alice user stat: %+v", data.UserStats[0])
	}
	if data.UserStats[1].Username != "bob" || data.UserStats[1].Quota != 200 || data.UserStats[1].ConsumeCount != 1 {
		t.Fatalf("unexpected bob user stat: %+v", data.UserStats[1])
	}
}

func TestGetBatchConsumeLogsRejectsTooManyUsernames(t *testing.T) {
	setupLogControllerTestDB(t)

	usernames := make([]string, 0, maxBatchConsumeUsernames+1)
	for i := 0; i < maxBatchConsumeUsernames+1; i++ {
		usernames = append(usernames, fmt.Sprintf("user-%d", i))
	}

	body := map[string]any{
		"usernames": usernames,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/log/batch/consume", body, 1)

	GetBatchConsumeLogs(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 status, got %d", recorder.Code)
	}
	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected failure response")
	}
	if response.Message == "" {
		t.Fatalf("expected validation message for too many usernames")
	}
}
