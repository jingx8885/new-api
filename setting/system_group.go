package setting

import (
	"encoding/json"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var systemGroups = map[string]string{}
var systemGroupsMutex sync.RWMutex

func GetSystemGroupsCopy() map[string]string {
	systemGroupsMutex.RLock()
	defer systemGroupsMutex.RUnlock()

	copyGroups := make(map[string]string, len(systemGroups))
	for key, value := range systemGroups {
		copyGroups[key] = value
	}
	return copyGroups
}

func SystemGroups2JSONString() string {
	systemGroupsMutex.RLock()
	defer systemGroupsMutex.RUnlock()

	jsonBytes, err := json.Marshal(systemGroups)
	if err != nil {
		common.SysLog("error marshalling system groups: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateSystemGroupsByJSONString(jsonStr string) error {
	systemGroupsMutex.Lock()
	defer systemGroupsMutex.Unlock()

	systemGroups = make(map[string]string)
	return json.Unmarshal([]byte(jsonStr), &systemGroups)
}
