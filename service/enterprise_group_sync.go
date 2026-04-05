package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/setting"
)

type EnterpriseGroupSyncPayload struct {
	IdentifierName string `json:"identifier_name"`
	CompanyName    string `json:"company_name"`
	Status         int    `json:"status"`
	Remark         string `json:"remark"`
}

func MergeEnterpriseManagedGroups(
	systemGroups map[string]string,
	userUsableGroups map[string]string,
	groupRatios map[string]float64,
	managedGroups map[string]setting.EnterpriseManagedGroup,
	payload []EnterpriseGroupSyncPayload,
) (map[string]string, map[string]string, map[string]float64, map[string]setting.EnterpriseManagedGroup, error) {
	return MergeEnterpriseManagedGroupsWithSystemGroups(
		systemGroups,
		userUsableGroups,
		groupRatios,
		managedGroups,
		payload,
	)
}

func MergeEnterpriseManagedGroupsWithSystemGroups(
	systemGroups map[string]string,
	userUsableGroups map[string]string,
	groupRatios map[string]float64,
	managedGroups map[string]setting.EnterpriseManagedGroup,
	payload []EnterpriseGroupSyncPayload,
) (map[string]string, map[string]string, map[string]float64, map[string]setting.EnterpriseManagedGroup, error) {
	nextSystemGroups := make(map[string]string, len(systemGroups))
	for key, value := range systemGroups {
		nextSystemGroups[key] = value
	}

	nextUserUsableGroups := make(map[string]string, len(userUsableGroups))
	for key, value := range userUsableGroups {
		nextUserUsableGroups[key] = value
	}

	nextGroupRatios := make(map[string]float64, len(groupRatios))
	for key, value := range groupRatios {
		nextGroupRatios[key] = value
	}

	currentManagedGroups := make(map[string]setting.EnterpriseManagedGroup, len(managedGroups))
	for key, value := range managedGroups {
		currentManagedGroups[key] = value
	}

	nextManagedGroups := make(map[string]setting.EnterpriseManagedGroup, len(payload))
	for _, item := range payload {
		identifierName := strings.TrimSpace(item.IdentifierName)
		if identifierName == "" {
			return nil, nil, nil, nil, fmt.Errorf("identifier_name 不能为空")
		}
		if _, exists := nextManagedGroups[identifierName]; exists {
			return nil, nil, nil, nil, fmt.Errorf("identifier_name 重复: %s", identifierName)
		}

		companyName := strings.TrimSpace(item.CompanyName)
		if companyName == "" {
			return nil, nil, nil, nil, fmt.Errorf("company_name 不能为空: %s", identifierName)
		}

		if _, managed := currentManagedGroups[identifierName]; !managed {
			if _, exists := nextSystemGroups[identifierName]; exists {
				return nil, nil, nil, nil, fmt.Errorf("标识名与 new-api 现有系统分组冲突: %s", identifierName)
			}
			if _, exists := nextUserUsableGroups[identifierName]; exists {
				return nil, nil, nil, nil, fmt.Errorf("标识名与 new-api 现有用户可选分组冲突: %s", identifierName)
			}
			if _, exists := nextGroupRatios[identifierName]; exists {
				return nil, nil, nil, nil, fmt.Errorf("标识名与 new-api 现有分组倍率冲突: %s", identifierName)
			}
		}

		nextSystemGroups[identifierName] = companyName
		nextUserUsableGroups[identifierName] = companyName
		if _, exists := nextGroupRatios[identifierName]; !exists {
			nextGroupRatios[identifierName] = 1
		}
		nextManagedGroups[identifierName] = setting.EnterpriseManagedGroup{
			CompanyName: companyName,
			Status:      item.Status,
			Remark:      strings.TrimSpace(item.Remark),
		}
	}

	for identifierName := range currentManagedGroups {
		if _, exists := nextManagedGroups[identifierName]; !exists {
			delete(nextSystemGroups, identifierName)
			delete(nextUserUsableGroups, identifierName)
			delete(nextGroupRatios, identifierName)
		}
	}

	return nextSystemGroups, nextUserUsableGroups, nextGroupRatios, nextManagedGroups, nil
}
