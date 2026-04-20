package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLogFilterTestDB(t *testing.T) {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalLogSQLType := common.LogSqlType
	originalCommonGroupCol := commonGroupCol
	originalCommonKeyCol := commonKeyCol
	originalCommonTrueVal := commonTrueVal
	originalCommonFalseVal := commonFalseVal
	originalLogGroupCol := logGroupCol
	originalLogKeyCol := logKeyCol

	t.Cleanup(func() {
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.LogSqlType = originalLogSQLType
		commonGroupCol = originalCommonGroupCol
		commonKeyCol = originalCommonKeyCol
		commonTrueVal = originalCommonTrueVal
		commonFalseVal = originalCommonFalseVal
		logGroupCol = originalLogGroupCol
		logKeyCol = originalLogKeyCol
	})

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.LogSqlType = common.DatabaseTypeSQLite
	initCol()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&User{}, &Channel{}, &Log{}); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}
}

func insertLogFilterTestUser(t *testing.T, id int, username string, group string) {
	t.Helper()
	user := &User{
		Id:          id,
		Username:    username,
		Password:    "secret123",
		DisplayName: username,
		Group:       group,
	}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}
}

func insertLogFilterTestLog(t *testing.T, log *Log) {
	t.Helper()
	if err := LOG_DB.Create(log).Error; err != nil {
		t.Fatalf("failed to insert log: %v", err)
	}
}

func insertLogFilterTestChannel(t *testing.T, id int, name string) {
	t.Helper()
	channel := &Channel{
		Id:          id,
		Name:        name,
		Key:         fmt.Sprintf("key-%d", id),
		Status:      common.ChannelStatusEnabled,
		Type:        1,
		Models:      "gpt-4o",
		Group:       "default",
		CreatedTime: 1,
	}
	if err := DB.Create(channel).Error; err != nil {
		t.Fatalf("failed to insert channel: %v", err)
	}
}

func TestParseChannelFilterExpression(t *testing.T) {
	t.Run("supports singles and ranges", func(t *testing.T) {
		filter, err := parseChannelFilterExpression("5, 8-10,20")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(filter.Singles) != 2 || filter.Singles[0] != 5 || filter.Singles[1] != 20 {
			t.Fatalf("unexpected singles: %#v", filter.Singles)
		}
		if len(filter.Ranges) != 1 || filter.Ranges[0].Start != 8 || filter.Ranges[0].End != 10 {
			t.Fatalf("unexpected ranges: %#v", filter.Ranges)
		}
	})

	t.Run("rejects invalid expression", func(t *testing.T) {
		invalidInputs := []string{
			"5,a",
			"10-5",
			"0",
			"3,",
		}
		for _, input := range invalidInputs {
			if _, err := parseChannelFilterExpression(input); err == nil {
				t.Fatalf("expected error for %q", input)
			}
		}
	})
}

func TestGetAllLogsSupportsChannelExpression(t *testing.T) {
	setupLogFilterTestDB(t)
	insertLogFilterTestUser(t, 1, "alice", "group-a")
	insertLogFilterTestChannel(t, 5, "channel-5")
	insertLogFilterTestChannel(t, 10, "channel-10")
	insertLogFilterTestChannel(t, 15, "channel-15")
	insertLogFilterTestChannel(t, 21, "channel-21")

	insertLogFilterTestLog(t, &Log{Id: 1, UserId: 1, Username: "alice", CreatedAt: 1001, Type: LogTypeConsume, TokenName: "token", ModelName: "gpt-4o", Quota: 5, ChannelId: 5, Group: "group-a"})
	insertLogFilterTestLog(t, &Log{Id: 2, UserId: 1, Username: "alice", CreatedAt: 1002, Type: LogTypeConsume, TokenName: "token", ModelName: "gpt-4o", Quota: 10, ChannelId: 10, Group: "group-a"})
	insertLogFilterTestLog(t, &Log{Id: 3, UserId: 1, Username: "alice", CreatedAt: 1003, Type: LogTypeConsume, TokenName: "token", ModelName: "gpt-4o", Quota: 15, ChannelId: 15, Group: "group-a"})
	insertLogFilterTestLog(t, &Log{Id: 4, UserId: 1, Username: "alice", CreatedAt: 1004, Type: LogTypeConsume, TokenName: "token", ModelName: "gpt-4o", Quota: 21, ChannelId: 21, Group: "group-a"})

	logs, total, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 20, "5,10-20", "", "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total=3, got %d", total)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}

	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", "5,10-20", "", "")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stat.Quota != 30 {
		t.Fatalf("expected quota=30, got %d", stat.Quota)
	}
}

func TestGroupFilterCanUseCurrentUserGroup(t *testing.T) {
	setupLogFilterTestDB(t)
	insertLogFilterTestUser(t, 1, "alice", "group-new")
	insertLogFilterTestChannel(t, 7, "channel-7")

	insertLogFilterTestLog(t, &Log{
		Id:        1,
		UserId:    1,
		Username:  "alice",
		CreatedAt: 2001,
		Type:      LogTypeConsume,
		TokenName: "token",
		ModelName: "gpt-4o",
		Quota:     100,
		ChannelId: 7,
		Group:     "group-old",
	})

	legacyLogs, legacyTotal, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 20, "", "group-new", "", "")
	if err != nil {
		t.Fatalf("legacy query returned error: %v", err)
	}
	if legacyTotal != 0 || len(legacyLogs) != 0 {
		t.Fatalf("expected legacy group filter to miss old log, total=%d len=%d", legacyTotal, len(legacyLogs))
	}

	currentLogs, currentTotal, err := GetAllLogs(LogTypeConsume, 0, 0, "", "", "", 0, 20, "", "group-new", LogGroupModeCurrentUser, "")
	if err != nil {
		t.Fatalf("current-user group query returned error: %v", err)
	}
	if currentTotal != 1 || len(currentLogs) != 1 {
		t.Fatalf("expected current-user group filter to match old log, total=%d len=%d", currentTotal, len(currentLogs))
	}
	if currentLogs[0].Group != "group-new" {
		t.Fatalf("expected returned log group to be rewritten to current group, got %q", currentLogs[0].Group)
	}

	userLogs, userTotal, err := GetUserLogs(1, LogTypeConsume, 0, 0, "", "", 0, 20, "group-new", LogGroupModeCurrentUser, "")
	if err != nil {
		t.Fatalf("user logs query returned error: %v", err)
	}
	if userTotal != 1 || len(userLogs) != 1 {
		t.Fatalf("expected user logs query to match old log, total=%d len=%d", userTotal, len(userLogs))
	}
	if userLogs[0].Group != "group-new" {
		t.Fatalf("expected user log group to be rewritten to current group, got %q", userLogs[0].Group)
	}

	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", "", "group-new", LogGroupModeCurrentUser)
	if err != nil {
		t.Fatalf("current-user stat returned error: %v", err)
	}
	if stat.Quota != 100 {
		t.Fatalf("expected quota=100, got %d", stat.Quota)
	}
}
