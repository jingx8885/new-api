package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type logAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type logPageResponse struct {
	Total int         `json:"total"`
	Items []model.Log `json:"items"`
}

type logStatResponse struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func setupLogControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate log controller test tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedLogControllerData(t *testing.T, db *gorm.DB) {
	t.Helper()

	users := []*model.User{
		{Id: 1, Username: "alice", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "aff-alice"},
		{Id: 2, Username: "alice-beta", Password: "password123", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "aff-alice-beta"},
	}
	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to create user %s: %v", user.Username, err)
		}
	}

	channels := []*model.Channel{
		{Id: 11, Name: "openai-main", Key: "sk-openai", Status: common.ChannelStatusEnabled, Group: "default"},
		{Id: 12, Name: "claude-cache", Key: "sk-claude", Status: common.ChannelStatusEnabled, Group: "default"},
	}
	for _, channel := range channels {
		if err := db.Create(channel).Error; err != nil {
			t.Fatalf("failed to create channel %s: %v", channel.Name, err)
		}
	}

	tokens := []*model.Token{
		{Id: 101, UserId: 1, Name: "alice-main", Key: "sk-alice-main", Status: common.TokenStatusEnabled, Group: "default", UnlimitedQuota: true, ExpiredTime: -1},
		{Id: 102, UserId: 2, Name: "alice-beta-main", Key: "sk-alice-beta", Status: common.TokenStatusEnabled, Group: "default", UnlimitedQuota: true, ExpiredTime: -1},
	}
	for _, token := range tokens {
		if err := db.Create(token).Error; err != nil {
			t.Fatalf("failed to create token %s: %v", token.Name, err)
		}
	}

	logs := []*model.Log{
		{
			UserId:           1,
			CreatedAt:        1710000000,
			Type:             model.LogTypeConsume,
			Content:          "输入 320 / 输出 88 / 缓存读 64 / 缓存写 16 / 用时 1.2s / 首字 210ms",
			Username:         "alice",
			TokenName:        "alice-main",
			ModelName:        "gpt-4o",
			Quota:            1250,
			PromptTokens:     320,
			CompletionTokens: 88,
			UseTime:          1,
			IsStream:         true,
			ChannelId:        11,
			TokenId:          101,
			Group:            "default",
			RequestId:        "req-alice-1",
			Other:            `{"model_ratio":1.25,"group_ratio":1.1,"cache_tokens":64,"cache_creation_tokens":16,"model_price":-1}`,
		},
		{
			UserId:           1,
			CreatedAt:        1710000300,
			Type:             model.LogTypeConsume,
			Content:          "输入 640 / 输出 144 / 缓存读 128 / 缓存写 32 / 用时 2.4s / 首字 320ms",
			Username:         "alice",
			TokenName:        "alice-main",
			ModelName:        "claude-3-7-sonnet",
			Quota:            2300,
			PromptTokens:     640,
			CompletionTokens: 144,
			UseTime:          2,
			IsStream:         true,
			ChannelId:        12,
			TokenId:          101,
			Group:            "default",
			RequestId:        "req-alice-2",
			Other:            `{"claude":true,"model_ratio":2.0,"group_ratio":1.0,"cache_tokens":128,"cache_creation_tokens":32,"model_price":-1}`,
		},
		{
			UserId:           2,
			CreatedAt:        1710000400,
			Type:             model.LogTypeConsume,
			Content:          "输入 111 / 输出 22 / 用时 0.8s / 首字 180ms",
			Username:         "alice-beta",
			TokenName:        "alice-beta-main",
			ModelName:        "gpt-4o",
			Quota:            999,
			PromptTokens:     111,
			CompletionTokens: 22,
			UseTime:          1,
			IsStream:         true,
			ChannelId:        11,
			TokenId:          102,
			Group:            "default",
			RequestId:        "req-alice-beta-1",
			Other:            `{"model_ratio":1.0,"group_ratio":1.0,"model_price":-1}`,
		},
		{
			UserId:    1,
			CreatedAt: 1710000500,
			Type:      model.LogTypeRefund,
			Content:   "任务失败退款",
			Username:  "alice",
			ModelName: "gpt-4o",
			Quota:     700,
			ChannelId: 11,
			TokenId:   101,
			Group:     "default",
			RequestId: "req-alice-refund-1",
			Other:     `{"reason":"task failed"}`,
		},
	}
	for _, entry := range logs {
		if err := db.Create(entry).Error; err != nil {
			t.Fatalf("failed to create log %s: %v", entry.RequestId, err)
		}
	}
}

func newLogTestContext(method string, target string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, nil)
	return ctx, recorder
}

func decodeLogResponse[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var response logAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode log api response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var data T
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode log response data: %v", err)
	}
	return data
}

func TestGetAllLogsFiltersConsumeRecordsByExactUsernameAndTimeRange(t *testing.T) {
	db := setupLogControllerTestDB(t)
	seedLogControllerData(t, db)

	ctx, recorder := newLogTestContext(
		http.MethodGet,
		"/api/log/?p=1&page_size=20&type=2&username=alice&start_timestamp=1709999900&end_timestamp=1710000350",
	)
	GetAllLogs(ctx)

	page := decodeLogResponse[logPageResponse](t, recorder)
	if page.Total != 2 {
		t.Fatalf("expected 2 consume logs for alice, got %d", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 returned items, got %d", len(page.Items))
	}
	if page.Items[0].Username != "alice" || page.Items[1].Username != "alice" {
		t.Fatalf("expected exact username filter to keep only alice logs, got %+v", page.Items)
	}
	if page.Items[0].RequestId != "req-alice-2" || page.Items[1].RequestId != "req-alice-1" {
		t.Fatalf("expected newest alice logs in desc order, got %+v", page.Items)
	}
	if page.Items[0].ChannelName != "claude-cache" || page.Items[1].ChannelName != "openai-main" {
		t.Fatalf("expected channel names to be enriched from channels table, got %+v", page.Items)
	}
}

func TestGetLogsStatMatchesConsumeQuotaForExactUsername(t *testing.T) {
	db := setupLogControllerTestDB(t)
	seedLogControllerData(t, db)

	ctx, recorder := newLogTestContext(
		http.MethodGet,
		"/api/log/stat?type=2&username=alice&start_timestamp=1709999900&end_timestamp=1710000600",
	)
	GetLogsStat(ctx)

	stat := decodeLogResponse[logStatResponse](t, recorder)
	if stat.Quota != 3550 {
		t.Fatalf("expected consume quota 3550 for alice, got %d", stat.Quota)
	}
	if stat.Quota == 4549 {
		t.Fatalf("stat unexpectedly included alice-beta records")
	}
}
