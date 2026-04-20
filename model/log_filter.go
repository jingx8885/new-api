package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const LogGroupModeCurrentUser = "current_user"

type channelRange struct {
	Start int
	End   int
}

type channelFilter struct {
	Singles []int
	Ranges  []channelRange
}

func parseChannelFilterExpression(expression string) (*channelFilter, error) {
	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	filter := &channelFilter{
		Singles: make([]int, 0, len(parts)),
		Ranges:  make([]channelRange, 0, len(parts)),
	}
	seenSingles := make(map[int]struct{}, len(parts))

	for _, rawPart := range parts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			return nil, errors.New("渠道 ID 格式错误，不能为空段")
		}
		if strings.Contains(part, "-") {
			bounds := strings.Split(part, "-")
			if len(bounds) != 2 {
				return nil, fmt.Errorf("渠道 ID 格式错误: %s", part)
			}
			start, err := parsePositiveChannelID(bounds[0])
			if err != nil {
				return nil, err
			}
			end, err := parsePositiveChannelID(bounds[1])
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, fmt.Errorf("渠道 ID 区间格式错误: %s", part)
			}
			filter.Ranges = append(filter.Ranges, channelRange{Start: start, End: end})
			continue
		}

		value, err := parsePositiveChannelID(part)
		if err != nil {
			return nil, err
		}
		if _, exists := seenSingles[value]; exists {
			continue
		}
		seenSingles[value] = struct{}{}
		filter.Singles = append(filter.Singles, value)
	}

	if len(filter.Singles) == 0 && len(filter.Ranges) == 0 {
		return nil, errors.New("渠道 ID 不能为空")
	}
	return filter, nil
}

func parsePositiveChannelID(raw string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("渠道 ID 格式错误: %s", strings.TrimSpace(raw))
	}
	return value, nil
}

func applyChannelFilter(tx *gorm.DB, column string, expression string) (*gorm.DB, error) {
	filter, err := parseChannelFilterExpression(expression)
	if err != nil || filter == nil {
		return tx, err
	}

	conditions := make([]string, 0, len(filter.Singles)+len(filter.Ranges))
	args := make([]interface{}, 0, len(filter.Singles)+len(filter.Ranges)*2)
	for _, single := range filter.Singles {
		conditions = append(conditions, column+" = ?")
		args = append(args, single)
	}
	for _, rng := range filter.Ranges {
		conditions = append(conditions, column+" BETWEEN ? AND ?")
		args = append(args, rng.Start, rng.End)
	}
	if len(conditions) == 0 {
		return tx, nil
	}
	return tx.Where("("+strings.Join(conditions, " OR ")+")", args...), nil
}

func applyLogGroupFilter(tx *gorm.DB, group string, groupMode string) *gorm.DB {
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return tx
	}
	if useCurrentUserGroupMode(groupMode, trimmedGroup) {
		return tx.Joins("LEFT JOIN users ON users.id = logs.user_id").
			Where(currentUserGroupExpression()+" = ?", trimmedGroup)
	}
	return tx.Where("logs."+logGroupCol+" = ?", trimmedGroup)
}

func rewriteLogsGroupForCurrentUserMode(logs []*Log, group string, groupMode string) {
	if !useCurrentUserGroupMode(groupMode, group) {
		return
	}
	for _, log := range logs {
		if log != nil {
			log.Group = strings.TrimSpace(group)
		}
	}
}

func useCurrentUserGroupMode(groupMode string, group string) bool {
	return strings.TrimSpace(group) != "" && strings.EqualFold(strings.TrimSpace(groupMode), LogGroupModeCurrentUser)
}

func currentUserGroupExpression() string {
	return fmt.Sprintf(
		"COALESCE(%s, %s)",
		qualifyQuotedColumn("users", commonGroupCol),
		qualifyQuotedColumn("logs", logGroupCol),
	)
}

func qualifyQuotedColumn(tableAlias string, quotedColumn string) string {
	column := strings.Trim(quotedColumn, "`\"")
	if common.UsingPostgreSQL {
		return fmt.Sprintf(`"%s"."%s"`, tableAlias, column)
	}
	return fmt.Sprintf("`%s`.`%s`", tableAlias, column)
}
