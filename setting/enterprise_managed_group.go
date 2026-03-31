package setting

import (
	"encoding/json"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

type EnterpriseManagedGroup struct {
	CompanyName string `json:"company_name"`
	Status      int    `json:"status"`
	Remark      string `json:"remark,omitempty"`
}

var enterpriseManagedGroups = map[string]EnterpriseManagedGroup{}
var enterpriseManagedGroupsMutex sync.RWMutex

func GetEnterpriseManagedGroupsCopy() map[string]EnterpriseManagedGroup {
	enterpriseManagedGroupsMutex.RLock()
	defer enterpriseManagedGroupsMutex.RUnlock()

	copyGroups := make(map[string]EnterpriseManagedGroup, len(enterpriseManagedGroups))
	for key, value := range enterpriseManagedGroups {
		copyGroups[key] = value
	}
	return copyGroups
}

func EnterpriseManagedGroups2JSONString() string {
	enterpriseManagedGroupsMutex.RLock()
	defer enterpriseManagedGroupsMutex.RUnlock()

	jsonBytes, err := json.Marshal(enterpriseManagedGroups)
	if err != nil {
		common.SysLog("error marshalling enterprise managed groups: " + err.Error())
	}
	return string(jsonBytes)
}

func UpdateEnterpriseManagedGroupsByJSONString(jsonStr string) error {
	enterpriseManagedGroupsMutex.Lock()
	defer enterpriseManagedGroupsMutex.Unlock()

	enterpriseManagedGroups = make(map[string]EnterpriseManagedGroup)
	return json.Unmarshal([]byte(jsonStr), &enterpriseManagedGroups)
}
