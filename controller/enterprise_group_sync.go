package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

const enterpriseSyncSecretHeader = "X-Enterprise-Sync-Secret"
const enterpriseSyncSecretEnv = "ENTERPRISE_SYNC_SECRET"

type enterpriseGroupSyncRequest struct {
	Groups []service.EnterpriseGroupSyncPayload `json:"groups"`
}

func SyncEnterpriseGroups(c *gin.Context) {
	expectedSecret := strings.TrimSpace(common.GetEnvOrDefaultString(enterpriseSyncSecretEnv, ""))
	if expectedSecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "enterprise sync secret is not configured",
		})
		return
	}

	receivedSecret := strings.TrimSpace(c.GetHeader(enterpriseSyncSecretHeader))
	if receivedSecret == "" || receivedSecret != expectedSecret {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "invalid enterprise sync secret",
		})
		return
	}

	var req enterpriseGroupSyncRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}

	nextSystemGroups, nextUserUsableGroups, nextGroupRatios, nextManagedGroups, err := service.MergeEnterpriseManagedGroupsWithSystemGroups(
		setting.GetSystemGroupsCopy(),
		setting.GetUserUsableGroupsCopy(),
		ratio_setting.GetGroupRatioCopy(),
		setting.GetEnterpriseManagedGroupsCopy(),
		req.Groups,
	)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if err = model.UpdateOptions(map[string]string{
		"SystemGroups":            common.GetJsonString(nextSystemGroups),
		"UserUsableGroups":        common.GetJsonString(nextUserUsableGroups),
		"GroupRatio":              common.GetJsonString(nextGroupRatios),
		"EnterpriseManagedGroups": common.GetJsonString(nextManagedGroups),
	}); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "同步企业分组失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
