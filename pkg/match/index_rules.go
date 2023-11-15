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

package match

import (
	"sort"

	"github.com/dynatrace/dynatrace-configuration-as-code/internal/log"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/rules"
)

// ByWeightTypeValue implements sort.Interface for []IndexRule based on
// the WeightTypeValue field.
type ByWeightTypeValue []rules.IndexRuleType

func (a ByWeightTypeValue) Len() int           { return len(a) }
func (a ByWeightTypeValue) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByWeightTypeValue) Less(i, j int) bool { return a[j].WeightValue < a[i].WeightValue }

type IndexRuleMapGenerator struct {
	SelfMatch    bool
	baseRuleList []rules.IndexRuleType
}

func NewIndexRuleMapGenerator(selfMatch bool, ruleList []rules.IndexRuleType) *IndexRuleMapGenerator {
	i := new(IndexRuleMapGenerator)
	i.SelfMatch = selfMatch
	i.baseRuleList = ruleList
	return i
}

func (i *IndexRuleMapGenerator) genActiveList() []rules.IndexRuleType {

	activeList := make([]rules.IndexRuleType, 0, len(i.baseRuleList))

	for idx, confType := range i.baseRuleList {
		ruleType := rules.IndexRuleType{
			Key:         idx,
			IsSeed:      confType.IsSeed,
			SplitMatch:  confType.SplitMatch,
			WeightValue: confType.WeightValue,
			IndexRules:  make([]rules.IndexRule, 0, len(confType.IndexRules)),
		}
		for _, conf := range confType.IndexRules {
			if conf.SelfMatchDisabled && i.SelfMatch {
				continue
			}
			ruleType.IndexRules = append(ruleType.IndexRules, conf)
		}
		if len(ruleType.IndexRules) >= 1 {
			activeList = append(activeList, ruleType)
		}
	}

	return activeList
}

func (i *IndexRuleMapGenerator) genSortedActiveList() []rules.IndexRuleType {

	activeList := i.genActiveList()

	sort.Sort(ByWeightTypeValue(activeList))

	return activeList
}

func runIndexRule(indexRule rules.IndexRule, indexRuleType rules.IndexRuleType, entityProcessingPtr *MatchProcessing, resultListPtr *IndexCompareResultList) bool {

	countsTowardsMax := false

	sortedIndexSource := genSortedItemsIndex(indexRule, &(*entityProcessingPtr).Source)
	sortedIndexTarget := genSortedItemsIndex(indexRule, &(*entityProcessingPtr).Target)

	needsPostProcessing := compareIndexes(resultListPtr, sortedIndexSource, sortedIndexTarget, indexRule, indexRuleType)

	if needsPostProcessing {
		countsTowardsMax = false
	} else if len(sortedIndexSource) > 0 || len(sortedIndexTarget) > 0 {
		countsTowardsMax = true
	}

	return countsTowardsMax
}

func keepMatches(matchedEntities map[int]int, uniqueMatch []CompareResult) map[int]int {
	for _, result := range uniqueMatch {
		_, found := matchedEntities[result.LeftId]

		if found {
			log.Error("Should never find multiple exact matches for an entity, %v", result)
		}

		matchedEntities[result.LeftId] = result.RightId
	}

	return matchedEntities
}

func (i *IndexRuleMapGenerator) RunIndexRuleAll(matchProcessingPtr *MatchProcessing) (*IndexCompareResultList, *map[int]int) {
	matchedEntities := map[int]int{}
	remainingResultsPtr := &IndexCompareResultList{}

	ruleTypes := i.genSortedActiveList()

	log.Info("Type: %s -> source count %d and target count %d", matchProcessingPtr.GetType(),
		matchProcessingPtr.Source.RawMatchList.Len(), matchProcessingPtr.Target.RawMatchList.Len())

	for _, indexRuleType := range ruleTypes {
		resultListPtr := newIndexCompareResultList(indexRuleType)
		matchProcessingPtr.PrepareRemainingMatch(true, indexRuleType.IsSeed, remainingResultsPtr)

		maxMatchValue := 0
		for _, indexRule := range indexRuleType.IndexRules {
			countsTowardsMax := runIndexRule(indexRule, indexRuleType, matchProcessingPtr, resultListPtr)
			if countsTowardsMax {
				maxMatchValue += indexRule.WeightValue
			}
		}

		resultListPtr.MergeRemainingWeightType(remainingResultsPtr)

		uniqueMatchEntities := resultListPtr.ProcessMatches(indexRuleType.SplitMatch, maxMatchValue)
		remainingResultsPtr = resultListPtr

		matchProcessingPtr.adjustremainingMatch(&uniqueMatchEntities)

		matchedEntities = keepMatches(matchedEntities, uniqueMatchEntities)
	}

	log.Info("Type: %s -> source count %d and target count %d -> Matched: %d",
		matchProcessingPtr.GetType(), len(*matchProcessingPtr.Source.RawMatchList.GetValues()),
		len(*matchProcessingPtr.Target.RawMatchList.GetValues()), len(matchedEntities))

	return remainingResultsPtr, &matchedEntities
}
