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
	"fmt"
	"path"
	"path/filepath"

	"github.com/dynatrace-oss/terraform-provider-dynatrace/dynatrace/settings"
	"github.com/dynatrace-oss/terraform-provider-dynatrace/dynatrace/settings/services/cache"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/log"
	config "github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/rules"
	"github.com/spf13/afero"
)

func genMultiMatchedMap(remainingResultsPtr *match.IndexCompareResultList, configProcessingPtr *match.MatchProcessing) map[string][]string {

	multiMatched := map[string][]string{}

	if len(remainingResultsPtr.CompareResults) <= 0 {
		return multiMatched
	}

	firstIdx := 0
	currentId := remainingResultsPtr.CompareResults[0].LeftId

	addMatchingMultiMatched := func(matchCount int) {
		multiMatchedMatches := make([]string, matchCount)
		for j := 0; j < matchCount; j++ {
			compareResult := remainingResultsPtr.CompareResults[(j + firstIdx)]
			targetId := compareResult.RightId

			multiMatchedMatches[j] = (*configProcessingPtr.Target.RawMatchList.GetValues())[targetId].(map[string]interface{})[rules.ConfigIdKey].(string)
		}
		multiMatched[(*configProcessingPtr.Source.RawMatchList.GetValues())[currentId].(map[string]interface{})[rules.ConfigIdKey].(string)] = multiMatchedMatches
	}

	for i := 1; i < len(remainingResultsPtr.CompareResults); i++ {
		result := remainingResultsPtr.CompareResults[i]
		if result.LeftId != currentId {
			matchCount := i - firstIdx
			addMatchingMultiMatched(matchCount)

			currentId = result.LeftId
			firstIdx = i
		}
	}
	matchCount := len(remainingResultsPtr.CompareResults) - firstIdx
	addMatchingMultiMatched(matchCount)

	return multiMatched

}

func printMultiMatchedSample(remainingResultsPtr *match.IndexCompareResultList, configProcessingPtr *match.MatchProcessing) {
	multiMatchedCount := len(remainingResultsPtr.CompareResults)

	if multiMatchedCount <= 0 {
		return
	}

	var maxPrint int
	if multiMatchedCount > 10 {
		maxPrint = 10
	} else {
		maxPrint = multiMatchedCount
	}

	for i := 0; i < maxPrint; i++ {
		result := remainingResultsPtr.CompareResults[i]
		log.Debug("Left: %v, Source: %v, Target: %v", result,
			(*configProcessingPtr.Source.RawMatchList.GetValues())[result.LeftId],
			(*configProcessingPtr.Target.RawMatchList.GetValues())[result.RightId])
	}

}

func getMultiMatched(remainingResultsPtr *match.IndexCompareResultList, configProcessingPtr *match.MatchProcessing) map[string][]string {
	printMultiMatchedSample(remainingResultsPtr, configProcessingPtr)

	return genMultiMatchedMap(remainingResultsPtr, configProcessingPtr)

}

type MatchOutputType struct {
	Type         string              `json:"type"`
	MatchKey     MatchKey            `json:"matchKey"`
	Matches      map[string]string   `json:"matches"`
	MultiMatched map[string][]string `json:"multiMatched"`
	UnMatched    []string            `json:"unmatched"`
}

type MatchKey struct {
	Source ExtractionInfo `json:"source"`
	Target ExtractionInfo `json:"target"`
}

type ExtractionInfo struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func genOutputPayload(configProcessingPtr *match.MatchProcessing, remainingResultsPtr *match.IndexCompareResultList, matchedConfigs *map[int]int) MatchOutputType {

	multiMatchedMap := getMultiMatched(remainingResultsPtr, configProcessingPtr)
	configProcessingPtr.PrepareRemainingMatch(false, true, remainingResultsPtr)

	// TODO: Decide if we need a From/To for configs or only configs
	matchOutput := MatchOutputType{
		Type: configProcessingPtr.GetType(),
		MatchKey: MatchKey{
			Source: ExtractionInfo{
				//From: (*configProcessingPtr).Source.ConfigType.From,
				//To:   (*configProcessingPtr).Source.ConfigType.To,
			},
			Target: ExtractionInfo{
				//From: (*configProcessingPtr).Target.ConfigType.From,
				//To:   (*configProcessingPtr).Target.ConfigType.To,
			},
		},
		Matches:      make(map[string]string, len(*matchedConfigs)),
		MultiMatched: multiMatchedMap,
		UnMatched:    make([]string, len(*configProcessingPtr.Source.CurrentRemainingMatch)),
	}

	for sourceI, targetI := range *matchedConfigs {
		matchOutput.Matches[(*configProcessingPtr.Source.RawMatchList.GetValues())[sourceI].(map[string]interface{})[rules.ConfigIdKey].(string)] =
			(*configProcessingPtr.Target.RawMatchList.GetValues())[targetI].(map[string]interface{})[rules.ConfigIdKey].(string)
	}

	for idx, sourceI := range *configProcessingPtr.Source.CurrentRemainingMatch {
		matchOutput.UnMatched[idx] = (*configProcessingPtr.Source.RawMatchList.GetValues())[sourceI].(map[string]interface{})[rules.ConfigIdKey].(string)
	}

	return matchOutput
}

func writeMatches(fs afero.Fs, configProcessingPtr *match.MatchProcessing, matchParameters match.MatchParameters, configsType string, configMatches MatchOutputType) error {
	sanitizedType := config.Sanitize(configsType)

	err := writeJsonMatchFile(fs, matchParameters.OutputDir, sanitizedType, configMatches)
	if err != nil {
		return err
	}

	err = writeTarFile(fs, matchParameters.OutputDir, sanitizedType, configProcessingPtr, configMatches)
	if err != nil {
		return err
	}

	return nil

}

func writeTarFile(fs afero.Fs, outputDir string, sanitizedType string, configProcessingPtr *match.MatchProcessing, configMatches MatchOutputType) error {
	tarDir := path.Join(outputDir, "cache")

	err := fs.MkdirAll(tarDir, 0777)
	if err != nil {
		return err
	}

	tarFolder, _, err := cache.NewTarFolder(path.Join(tarDir, sanitizedType))
	if err != nil {
		return err
	}

	for _, conf := range *configProcessingPtr.Source.RawMatchList.GetValues() {
		bytes, err := json.Marshal(conf)
		if err != nil {
			return err
		}

		modifiedStringPtr, configId, wasModified := replaceConfigs(&conf, &bytes, &configMatches)

		if wasModified {
			bytes = []byte(*modifiedStringPtr)
		}

		name, ok := conf.(map[string]interface{})[rules.ConfigNameKey].(string)
		if !ok {
			name = *configId
		}

		tarFolder.Save(
			settings.Stub{
				ID:   *configId,
				Name: name,
			},
			bytes,
		)
	}

	return nil
}

func writeJsonMatchFile(fs afero.Fs, outputDir string, sanitizedType string, output MatchOutputType) error {
	jsonDir := path.Join(outputDir, "dict")

	err := fs.MkdirAll(jsonDir, 0777)
	if err != nil {
		return err
	}

	outputAsJson, err := json.Marshal(output)
	if err != nil {
		return err
	}

	fullMatchPath := filepath.Join(jsonDir, fmt.Sprintf("%s.json", sanitizedType))

	err = afero.WriteFile(fs, fullMatchPath, outputAsJson, 0664)

	if err != nil {
		return err
	}

	return nil

}
