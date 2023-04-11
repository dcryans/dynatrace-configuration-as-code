// @license
// Copyright 2023 Dynatrace LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configs

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	//"github.com/dynatrace-oss/terraform-provider-dynatrace/dynatrace/settings"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/errutils"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/log"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/client"
	config "github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/download/classic"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/entities"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/rules"
	project "github.com/dynatrace/dynatrace-configuration-as-code/pkg/project/v2"
)

const SettingsIdKey = "objectId"

type RawConfigsList struct {
	Values *[]interface{}
}

// ByRawConfigId implements sort.Interface for []RawConfig] based on
// the ConfigId string field.
type ByRawConfigId []interface{}

func (a ByRawConfigId) Len() int      { return len(a) }
func (a ByRawConfigId) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByRawConfigId) Less(i, j int) bool {
	return (a[i].(map[string]interface{}))[rules.ConfigIdKey].(string) < (a[j].(map[string]interface{}))[rules.ConfigIdKey].(string)
}

func (r *RawConfigsList) Sort() {

	sort.Sort(ByRawConfigId(*r.GetValues()))

}

func (r *RawConfigsList) Len() int {

	return len(*r.GetValues())

}

func (r *RawConfigsList) GetValues() *[]interface{} {

	return r.Values

}

func unmarshalConfigs(configPerType []config.Config, configType config.Type, entityMatches entities.MatchOutputPerType) (*RawConfigsList, error) {
	rawConfigsList := &RawConfigsList{
		Values: new([]interface{}),
	}

	if len(configPerType) <= 0 {
		return rawConfigsList, nil
	}

	err := json.Unmarshal([]byte(configPerType[0].Template.Content()), rawConfigsList.Values)
	if err != nil {
		return nil, err
	}

	var configIdLocation string
	isSettings := configType.ID() == config.SettingsTypeId
	if isSettings {
		configIdLocation = SettingsIdKey
	} else {
		configIdLocation = classic.ClassicIdKey
	}

	errs := []error{}
	mutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(len(*rawConfigsList.Values))

	for i, conf := range *rawConfigsList.Values {
		go func(confIdx int, confInterface interface{}) {
			defer waitGroup.Done()
			confMap := confInterface.(map[string]interface{})[client.DownloadedKey].(map[string]interface{})

			entities, confInterfaceModified, err := extractReplaceEntities(confMap, entityMatches)
			if err != nil {
				mutex.Lock()
				errs = append(errs, err)
				mutex.Unlock()
				return
			}

			uniqueConfKey := ""
			uniqueConfOk := false
			var classicNameValue interface{}

			if isSettings {
				uniqueConfKey, uniqueConfOk = name(&confMap)
			} else {

				if configType.(config.ClassicApiType).Api == "dashboard" {
					classicNameValue = confMap["dashboardMetadata"].(map[string]interface{})["name"]
				} else {
					name, ok := confMap["name"]
					if ok {
						classicNameValue = name
					} else if displayName, ok := confMap["displayName"]; ok {
						classicNameValue = displayName
					}
				}
			}

			mutex.Lock()
			if confInterfaceModified != nil {
				(*rawConfigsList.Values)[confIdx].(map[string]interface{})[client.DownloadedKey] = confInterfaceModified
			}
			(*rawConfigsList.Values)[confIdx].(map[string]interface{})[rules.ConfigIdKey] = confMap[configIdLocation].(string)
			(*rawConfigsList.Values)[confIdx].(map[string]interface{})[rules.EntitiesListKey] = entities

			if uniqueConfOk {
				(*rawConfigsList.Values)[confIdx].(map[string]interface{})[rules.ConfigNameKey] = uniqueConfKey
			} else if classicNameValue != nil {
				(*rawConfigsList.Values)[confIdx].(map[string]interface{})[rules.ConfigNameKey] = classicNameValue
			}
			mutex.Unlock()
		}(i, conf)

	}

	waitGroup.Wait()

	if len(errs) >= 1 {
		return nil, errutils.PrintAndFormatErrors(errs, "failed to enhance configs with required fields")
	}

	return rawConfigsList, err
}

func genConfigProcessing(configPerTypeSource project.ConfigsPerType, configPerTypeTarget project.ConfigsPerType, configsType string, entityMatches entities.MatchOutputPerType) (*match.MatchProcessing, error) {

	startTime := time.Now()

	var sourceType config.Type
	if len(configPerTypeSource[configsType]) > 0 {
		sourceType = configPerTypeSource[configsType][0].Type
	}

	//rawConfigsSource, err := convertConfigSliceToRawList(configPerTypeSource[configsType])
	rawConfigsSource, err := unmarshalConfigs(configPerTypeSource[configsType], sourceType, entityMatches)
	if err != nil {
		return nil, err
	}

	var targetType config.Type
	if len(configPerTypeTarget[configsType]) > 0 {
		targetType = configPerTypeTarget[configsType][0].Type
	}

	//rawConfigsTarget, err := convertConfigSliceToRawList(configPerTypeTarget[configsType])
	rawConfigsTarget, err := unmarshalConfigs(configPerTypeTarget[configsType], targetType, nil)
	if err != nil {
		return nil, err
	}

	log.Debug("Enhanced %s in %v", configsType, time.Since(startTime))

	return match.NewMatchProcessing(rawConfigsSource, sourceType, rawConfigsTarget, targetType), nil
}
