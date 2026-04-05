package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
)

func TestMergeEnterpriseManagedGroupsPreservesNativeGroups(t *testing.T) {
	systemGroups := map[string]string{
		"default": "默认分组",
		"native":  "原生系统分组",
		"acme":    "旧企业名",
	}
	userGroups := map[string]string{
		"default": "默认分组",
		"native":  "原生分组",
		"acme":    "旧企业名",
	}
	groupRatios := map[string]float64{
		"default": 1,
		"native":  1.5,
		"acme":    2.5,
	}
	managedGroups := map[string]setting.EnterpriseManagedGroup{
		"acme": {
			CompanyName: "旧企业名",
			Status:      1,
			Remark:      "old",
		},
	}

	mergedSystemGroups, mergedUserGroups, mergedGroupRatios, mergedManagedGroups, err := MergeEnterpriseManagedGroupsWithSystemGroups(systemGroups, userGroups, groupRatios, managedGroups, []EnterpriseGroupSyncPayload{
		{
			IdentifierName: "acme",
			CompanyName:    "新企业名",
			Status:         1,
			Remark:         "new",
		},
		{
			IdentifierName: "beta",
			CompanyName:    "Beta 企业",
			Status:         0,
			Remark:         "beta",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mergedUserGroups["default"] != "默认分组" {
		t.Fatalf("expected default group to be preserved")
	}
	if mergedUserGroups["native"] != "原生分组" {
		t.Fatalf("expected native group to be preserved")
	}
	if mergedSystemGroups["native"] != "原生系统分组" {
		t.Fatalf("expected native system group to be preserved")
	}
	if mergedSystemGroups["acme"] != "新企业名" {
		t.Fatalf("expected managed system group to update display name")
	}
	if mergedUserGroups["acme"] != "新企业名" {
		t.Fatalf("expected managed group to update display name")
	}
	if mergedUserGroups["beta"] != "Beta 企业" {
		t.Fatalf("expected new managed group to be inserted")
	}
	if mergedSystemGroups["beta"] != "Beta 企业" {
		t.Fatalf("expected new managed system group to be inserted")
	}
	if mergedGroupRatios["acme"] != 2.5 {
		t.Fatalf("expected existing managed group ratio to be preserved")
	}
	if mergedGroupRatios["beta"] != 1 {
		t.Fatalf("expected new managed group ratio to default to 1")
	}
	if mergedGroupRatios["native"] != 1.5 {
		t.Fatalf("expected native group ratio to be preserved")
	}
	if len(mergedManagedGroups) != 2 {
		t.Fatalf("expected 2 managed groups, got %d", len(mergedManagedGroups))
	}
}

func TestMergeEnterpriseManagedGroupsRemovesDeletedManagedGroupsOnly(t *testing.T) {
	systemGroups := map[string]string{
		"default": "默认分组",
		"native":  "原生系统分组",
		"acme":    "Acme 企业",
	}
	userGroups := map[string]string{
		"default": "默认分组",
		"native":  "原生分组",
		"acme":    "Acme 企业",
	}
	groupRatios := map[string]float64{
		"default": 1,
		"native":  1.2,
		"acme":    1.8,
	}
	managedGroups := map[string]setting.EnterpriseManagedGroup{
		"acme": {
			CompanyName: "Acme 企业",
			Status:      1,
		},
	}

	mergedSystemGroups, mergedUserGroups, mergedGroupRatios, mergedManagedGroups, err := MergeEnterpriseManagedGroupsWithSystemGroups(systemGroups, userGroups, groupRatios, managedGroups, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if _, exists := mergedSystemGroups["acme"]; exists {
		t.Fatalf("expected deleted managed system group to be removed")
	}
	if _, exists := mergedUserGroups["acme"]; exists {
		t.Fatalf("expected deleted managed group to be removed")
	}
	if _, exists := mergedGroupRatios["acme"]; exists {
		t.Fatalf("expected deleted managed group ratio to be removed")
	}
	if mergedUserGroups["native"] != "原生分组" {
		t.Fatalf("expected native group to remain")
	}
	if mergedSystemGroups["native"] != "原生系统分组" {
		t.Fatalf("expected native system group to remain")
	}
	if len(mergedManagedGroups) != 0 {
		t.Fatalf("expected managed group registry to be empty")
	}
}

func TestMergeEnterpriseManagedGroupsRejectsConflictWithNativeGroup(t *testing.T) {
	systemGroups := map[string]string{
		"default": "默认分组",
		"native":  "原生系统分组",
	}
	userGroups := map[string]string{
		"default": "默认分组",
		"native":  "原生分组",
	}

	_, _, _, _, err := MergeEnterpriseManagedGroupsWithSystemGroups(systemGroups, userGroups, map[string]float64{"default": 1, "native": 1}, map[string]setting.EnterpriseManagedGroup{}, []EnterpriseGroupSyncPayload{
		{
			IdentifierName: "native",
			CompanyName:    "会冲突",
			Status:         1,
		},
	})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
}

func TestMergeEnterpriseManagedGroupsRejectsConflictWithNativeUserUsableGroup(t *testing.T) {
	_, _, _, _, err := MergeEnterpriseManagedGroupsWithSystemGroups(
		map[string]string{"default": "默认系统分组"},
		map[string]string{"default": "默认分组", "native_usable": "原生可选分组"},
		map[string]float64{"default": 1},
		map[string]setting.EnterpriseManagedGroup{},
		[]EnterpriseGroupSyncPayload{
			{
				IdentifierName: "native_usable",
				CompanyName:    "冲突可选组",
				Status:         1,
			},
		},
	)
	if err == nil {
		t.Fatalf("expected user usable group conflict error")
	}
}

func TestMergeEnterpriseManagedGroupsRejectsConflictWithNativeGroupRatio(t *testing.T) {
	_, _, _, _, err := MergeEnterpriseManagedGroupsWithSystemGroups(
		map[string]string{"default": "默认系统分组"},
		map[string]string{"default": "默认分组"},
		map[string]float64{"default": 1, "native_ratio": 1.3},
		map[string]setting.EnterpriseManagedGroup{},
		[]EnterpriseGroupSyncPayload{
			{
				IdentifierName: "native_ratio",
				CompanyName:    "冲突倍率组",
				Status:         1,
			},
		},
	)
	if err == nil {
		t.Fatalf("expected group ratio conflict error")
	}
}
