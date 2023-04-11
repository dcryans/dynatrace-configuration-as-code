/**
* @license
* Copyright 2020 Dynatrace LLC
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package configs

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/dynatrace/dynatrace-configuration-as-code/internal/log"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/entities"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/rules"
)

var entityExtractionRegex = regexp.MustCompile(`(((?:[A-Z]+_)?(?:[A-Z]+_)?(?:[A-Z]+_)?[A-Z]+)-[0-9A-Z]{16})`)

var main_object_key_regex_list = []*regexp.Regexp{
	regexp.MustCompile(`(^[Nn]ame$)`),
	regexp.MustCompile(`(^[Kk]ey$)`),
	regexp.MustCompile(`(^[Ss]ummary$)`),
	regexp.MustCompile(`(^[Ll]abel$)`),
	regexp.MustCompile(`(^[Tt]itle$)`),
	regexp.MustCompile(`(^[Pp]attern$)`),
	regexp.MustCompile(`(^[Rr]ule$)`),
	regexp.MustCompile(`(^.*[Nn]ame$)`),
	regexp.MustCompile(`(^.*[Kk]ey$)`),
	regexp.MustCompile(`(^.*[Ss]ummary$)`),
	regexp.MustCompile(`(^.*[Ll]abel$)`),
	regexp.MustCompile(`(^.*[Tt]itle$)`),
	regexp.MustCompile(`(^.*[Pp]attern$)`),
	regexp.MustCompile(`(^.*[Rr]ule$)`),
}

func name(v *map[string]interface{}) (string, bool) {

	for _, regex := range main_object_key_regex_list {
		for key, value := range (*v)["value"].(map[string]interface{}) {
			if regex.Match([]byte(key)) {
				stringValue, ok := value.(string)
				if ok && stringValue != "" {
					return value.(string), true

				}
			}
		}
	}

	return "", false
}

func extractReplaceEntities(confInterface map[string]interface{}, entityMatches entities.MatchOutputPerType) ([]interface{}, interface{}, error) {
	rawJson, err := json.Marshal(confInterface)
	if err != nil {
		return nil, nil, err
	}

	matches := entityExtractionRegex.FindAll(rawJson, -1)

	jsonString := string(rawJson)

	matchesStrings := make([]interface{}, len(matches))

	wasModified := false

	for i, bytes := range matches {
		entityId := string(bytes)
		matchesStrings[i] = entityId

		entityMatchType, ok := entityMatches[string(bytes[0:(len(bytes)-17)])]
		if ok {
			entityMatch, ok := entityMatchType.Matches[entityId]
			if ok {
				if entityMatch != entityId {
					jsonString = strings.ReplaceAll(jsonString, entityId, entityMatch)
					matchesStrings[i] = entityMatch
					wasModified = true
				}
			}
		}
	}

	if wasModified {
		var confInterfaceModified interface{}
		json.Unmarshal([]byte(jsonString), &confInterfaceModified)
		return matchesStrings, confInterfaceModified, nil
	}

	return matchesStrings, nil, nil

}

func replaceConfigs(confPtr *interface{}, confBytesPtr *[]byte, configMatchesPtr *MatchOutputType) (*string, *string, bool) {

	configId := (*confPtr).(map[string]interface{})[rules.ConfigIdKey].(string)
	if configId == "" {
		log.Error("CONFIG ID MISSING!!!")
	}

	configIdMatch, ok := configMatchesPtr.Matches[configId]
	if ok {
		if configIdMatch != configId {
			modifiedString := strings.ReplaceAll(string(*confBytesPtr), configId, configIdMatch)
			return &modifiedString, &configIdMatch, true
		}
	}

	return nil, &configId, false
}
