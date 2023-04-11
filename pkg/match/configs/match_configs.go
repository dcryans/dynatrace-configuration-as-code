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
	"fmt"
	"sync"

	"github.com/dynatrace/dynatrace-configuration-as-code/internal/errutils"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/log"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/entities"
	project "github.com/dynatrace/dynatrace-configuration-as-code/pkg/project/v2"
	"github.com/spf13/afero"
)

func MatchConfigs(fs afero.Fs, matchParameters match.MatchParameters, configPerTypeSource project.ConfigsPerType, configPerTypeTarget project.ConfigsPerType) ([]string, int, int, error) {
	configsSourceCount := 0
	configsTargetCount := 0
	stats := []string{fmt.Sprintf("%65s %10s %12s %10s %10s %10s", "Type", "Matched", "MultiMatched", "UnMatched", "Total", "Source")}
	errs := []error{}

	mutex := sync.Mutex{}
	waitGroup := sync.WaitGroup{}
	waitGroup.Add(len(configPerTypeTarget))

	for configsTypeLoop := range configPerTypeTarget {
		go func(configsType string) {
			defer waitGroup.Done()
			log.Debug("Processing Type: %s", configsType)

			entityMatches, err := entities.LoadMatches(fs, matchParameters)
			if err != nil {
				mutex.Lock()
				errs = append(errs, err)
				mutex.Unlock()
				return
			}

			configProcessingPtr, err := genConfigProcessing(configPerTypeSource, configPerTypeTarget, configsType, entityMatches)
			if err != nil {
				mutex.Lock()
				errs = append(errs, err)
				mutex.Unlock()
				return
			}

			configsSourceCountType := len(configProcessingPtr.Source.RemainingMatch)
			configsTargetCountType := len(configProcessingPtr.Target.RemainingMatch)

			configMatches := runRules(configProcessingPtr, matchParameters)

			err = writeMatches(fs, configProcessingPtr, matchParameters, configsType, configMatches)
			if err != nil {
				mutex.Lock()
				errs = append(errs, fmt.Errorf("failed to persist matches of type: %s, see error: %w", configsType, err))
				mutex.Unlock()
				return
			}

			mutex.Lock()
			configsSourceCount += configsSourceCountType
			configsTargetCount += configsTargetCountType
			stats = append(stats, fmt.Sprintf("%65s %10d %12d %10d %10d %10d", configsType, len(configMatches.Matches), len(configMatches.MultiMatched), len(configMatches.UnMatched), configsTargetCountType, configsSourceCountType))
			mutex.Unlock()
		}(configsTypeLoop)

	}

	waitGroup.Wait()

	if len(errs) >= 1 {
		return []string{}, 0, 0, errutils.PrintAndFormatErrors(errs, "failed to match configs with required fields")
	}

	return stats, configsSourceCount, configsTargetCount, nil
}
