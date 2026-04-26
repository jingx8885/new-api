package controller

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

const (
	channelBatchModelModeAppend  = "append"
	channelBatchModelModeReplace = "replace"
)

type ChannelBatchModelsRequest struct {
	Ids          []int    `json:"ids"`
	Models       []string `json:"models"`
	Mode         string   `json:"mode"`
	DryRun       bool     `json:"dry_run"`
	Test         bool     `json:"test"`
	TestModels   []string `json:"test_models"`
	EndpointType string   `json:"endpoint_type"`
	Stream       bool     `json:"stream"`
}

type ChannelBatchModelTestResult struct {
	Model     string  `json:"model"`
	Success   bool    `json:"success"`
	Message   string  `json:"message"`
	Time      float64 `json:"time"`
	ErrorCode string  `json:"error_code,omitempty"`
}

type ChannelBatchModelResult struct {
	ChannelID       int                           `json:"channel_id"`
	ChannelName     string                        `json:"channel_name"`
	BeforeModels    []string                      `json:"before_models"`
	RequestedModels []string                      `json:"requested_models"`
	AfterModels     []string                      `json:"after_models"`
	AddedModels     []string                      `json:"added_models"`
	MissingModels   []string                      `json:"missing_models"`
	Changed         bool                          `json:"changed"`
	UpdateSuccess   bool                          `json:"update_success"`
	UpdateMessage   string                        `json:"update_message"`
	Tests           []ChannelBatchModelTestResult `json:"tests"`
}

type ChannelBatchModelsResponse struct {
	Results       []ChannelBatchModelResult `json:"results"`
	ChannelCount  int                       `json:"channel_count"`
	FailedUpdates int                       `json:"failed_updates"`
	FailedTests   int                       `json:"failed_tests"`
	DryRun        bool                      `json:"dry_run"`
}

func normalizeChannelBatchModels(models []string) []string {
	normalized := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, modelName := range models {
		for _, part := range strings.Split(modelName, ",") {
			name := strings.TrimSpace(part)
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			normalized = append(normalized, name)
		}
	}
	return normalized
}

func mergeChannelBatchModels(existing []string, requested []string, mode string) ([]string, error) {
	switch mode {
	case "", channelBatchModelModeAppend:
		merged := append([]string{}, existing...)
		seen := make(map[string]struct{}, len(existing)+len(requested))
		for _, modelName := range existing {
			seen[modelName] = struct{}{}
		}
		for _, modelName := range requested {
			if _, ok := seen[modelName]; ok {
				continue
			}
			seen[modelName] = struct{}{}
			merged = append(merged, modelName)
		}
		return merged, nil
	case channelBatchModelModeReplace:
		return append([]string{}, requested...), nil
	default:
		return nil, fmt.Errorf("不支持的配置模式: %s", mode)
	}
}

func subtractChannelBatchModels(models []string, remove []string) []string {
	removeSet := make(map[string]struct{}, len(remove))
	for _, modelName := range remove {
		removeSet[modelName] = struct{}{}
	}
	result := make([]string, 0)
	for _, modelName := range models {
		if _, ok := removeSet[modelName]; !ok {
			result = append(result, modelName)
		}
	}
	return result
}

func intersectChannelBatchModels(models []string, candidates []string) []string {
	candidateSet := make(map[string]struct{}, len(candidates))
	for _, modelName := range candidates {
		candidateSet[modelName] = struct{}{}
	}
	result := make([]string, 0)
	for _, modelName := range models {
		if _, ok := candidateSet[modelName]; ok {
			result = append(result, modelName)
		}
	}
	return result
}

func buildChannelBatchTestResult(channel *model.Channel, modelName string, endpointType string, stream bool) ChannelBatchModelTestResult {
	tik := time.Now()
	result := testChannel(channel, modelName, endpointType, stream)
	consumedTime := float64(time.Since(tik).Milliseconds()) / 1000.0
	testResult := ChannelBatchModelTestResult{
		Model: modelName,
		Time:  consumedTime,
	}
	if result.localErr != nil {
		testResult.Success = false
		testResult.Message = result.localErr.Error()
		if result.newAPIError != nil {
			testResult.ErrorCode = string(result.newAPIError.GetErrorCode())
		}
		return testResult
	}
	if result.newAPIError != nil {
		testResult.Success = false
		testResult.Message = result.newAPIError.Error()
		testResult.ErrorCode = string(result.newAPIError.GetErrorCode())
		return testResult
	}
	testResult.Success = true
	return testResult
}

func BatchConfigChannelModels(c *gin.Context) {
	req := ChannelBatchModelsRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	requestedModels := normalizeChannelBatchModels(req.Models)
	if len(req.Ids) == 0 {
		common.ApiErrorMsg(c, "请选择要配置的渠道")
		return
	}
	if len(requestedModels) == 0 {
		common.ApiErrorMsg(c, "请输入要配置的模型")
		return
	}

	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = channelBatchModelModeAppend
	}
	testModels := normalizeChannelBatchModels(req.TestModels)
	if len(testModels) == 0 {
		testModels = requestedModels
	}

	resp := ChannelBatchModelsResponse{
		Results: make([]ChannelBatchModelResult, 0, len(req.Ids)),
		DryRun:  req.DryRun,
	}
	changed := false
	for _, channelID := range req.Ids {
		result := ChannelBatchModelResult{
			ChannelID:       channelID,
			RequestedModels: requestedModels,
			UpdateSuccess:   true,
			UpdateMessage:   "unchanged",
			Tests:           []ChannelBatchModelTestResult{},
		}

		channel, err := model.GetChannelById(channelID, true)
		if err != nil {
			result.UpdateSuccess = false
			result.UpdateMessage = err.Error()
			result.MissingModels = requestedModels
			resp.Results = append(resp.Results, result)
			resp.FailedUpdates++
			continue
		}

		result.ChannelName = channel.Name
		result.BeforeModels = normalizeChannelBatchModels(channel.GetModels())
		nextModels, err := mergeChannelBatchModels(result.BeforeModels, requestedModels, mode)
		if err != nil {
			result.UpdateSuccess = false
			result.UpdateMessage = err.Error()
			result.MissingModels = requestedModels
			resp.Results = append(resp.Results, result)
			resp.FailedUpdates++
			continue
		}
		result.AddedModels = subtractChannelBatchModels(requestedModels, result.BeforeModels)
		result.Changed = strings.Join(result.BeforeModels, ",") != strings.Join(nextModels, ",")

		if result.Changed {
			if req.DryRun {
				result.UpdateMessage = "dry-run"
			} else {
				channel.Models = strings.Join(nextModels, ",")
				if err := channel.Update(); err != nil {
					result.UpdateSuccess = false
					result.UpdateMessage = err.Error()
					result.MissingModels = requestedModels
					resp.Results = append(resp.Results, result)
					resp.FailedUpdates++
					continue
				}
				changed = true
				result.UpdateMessage = "updated"
			}
		}

		verifiedModels := nextModels
		if !req.DryRun {
			verifiedChannel, err := model.GetChannelById(channelID, true)
			if err != nil {
				result.UpdateSuccess = false
				result.UpdateMessage = err.Error()
				result.MissingModels = requestedModels
				resp.Results = append(resp.Results, result)
				resp.FailedUpdates++
				continue
			}
			channel = verifiedChannel
			verifiedModels = normalizeChannelBatchModels(verifiedChannel.GetModels())
		}
		result.AfterModels = verifiedModels
		result.MissingModels = subtractChannelBatchModels(requestedModels, verifiedModels)
		if len(result.MissingModels) > 0 {
			resp.FailedUpdates++
		}

		if req.Test && !req.DryRun && len(result.MissingModels) == 0 {
			modelsToTest := intersectChannelBatchModels(testModels, requestedModels)
			if len(modelsToTest) == 0 {
				modelsToTest = testModels
			}
			for _, modelName := range modelsToTest {
				testResult := buildChannelBatchTestResult(channel, modelName, req.EndpointType, req.Stream)
				if !testResult.Success {
					resp.FailedTests++
				}
				result.Tests = append(result.Tests, testResult)
			}
		}

		resp.Results = append(resp.Results, result)
	}

	if changed {
		model.InitChannelCache()
		service.ResetProxyClientCache()
	}
	resp.ChannelCount = len(resp.Results)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    resp,
	})
}
