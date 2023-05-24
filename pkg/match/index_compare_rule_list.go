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
)

type IndexCompareResultList struct {
	CompareResults []CompareResult
}

func newIndexCompareResultList() *IndexCompareResultList {
	i := new(IndexCompareResultList)
	i.CompareResults = []CompareResult{}
	return i
}

func newReversedIndexCompareResultList(sourceList *IndexCompareResultList) *IndexCompareResultList {
	i := new(IndexCompareResultList)
	size := len(sourceList.CompareResults)
	i.CompareResults = make([]CompareResult, size)
	resI := 0

	for _, result := range sourceList.CompareResults {
		i.CompareResults[resI] = CompareResult{result.RightId, result.LeftId, result.Weight}
		resI++
	}

	if resI != size {
		log.Error("Did not reverse properly!")
	}
	return i
}

func (i *IndexCompareResultList) addResult(entityIdSource int, entityIdTarget int, WeightValue int) {
	i.CompareResults = append(i.CompareResults, CompareResult{entityIdSource, entityIdTarget, WeightValue})
}

func (i *IndexCompareResultList) sortTopMatches() {

	sort.Sort(ByTopMatch(i.CompareResults))

}

func (i *IndexCompareResultList) keepTopMatchesOnly() {

	if len(i.CompareResults) == 0 {
		return
	}

	i.sortTopMatches()

	topMatchesResults := []CompareResult{}
	prevTop := i.CompareResults[0]

	for _, result := range i.CompareResults {

		if result.LeftId == prevTop.LeftId {
			if result.Weight != prevTop.Weight {
				continue
			}
		} else {
			prevTop = result
		}

		topMatchesResults = append(topMatchesResults, result)

	}

	i.CompareResults = topMatchesResults

}

func (i *IndexCompareResultList) reduceBothForwardAndBackward() *IndexCompareResultList {

	i.keepTopMatchesOnly()

	reverseResults := newReversedIndexCompareResultList(i)
	reverseResults.keepTopMatchesOnly()

	i.CompareResults = newReversedIndexCompareResultList(reverseResults).CompareResults

	return reverseResults
}

func (i *IndexCompareResultList) sort() {

	sort.Sort(ByLeftRight(i.CompareResults))

}

func (i *IndexCompareResultList) getUniqueMatchItems() []CompareResult {

	if len(i.CompareResults) == 0 {
		return []CompareResult{}
	}

	i.sort()

	uniqueMatchItems := []CompareResult{}

	prevResult := i.CompareResults[0]
	prevTotalSeen := 1

	extractUniqueMatch := func() {
		if prevTotalSeen == 1 {
			uniqueMatchItems = append(uniqueMatchItems, prevResult)
		}
	}

	for _, compareResult := range i.CompareResults[1:] {
		if compareResult.LeftId == prevResult.LeftId {
			prevTotalSeen += 1
		} else {
			extractUniqueMatch()
			prevResult = compareResult
			prevTotalSeen = 1
		}
	}
	extractUniqueMatch()

	return uniqueMatchItems
}

func (i *IndexCompareResultList) sumMatchWeightValues(splitMatch bool, maxMatchValue int) {

	if len(i.CompareResults) <= 1 {
		return
	}

	i.sort()

	summedMatchResults := make([]CompareResult, 0, len(i.CompareResults))
	prevTotal := i.CompareResults[0]

	aI := 0
	bI := 1

	var addRecord = func() {
		keepRecord := true
		if splitMatch && prevTotal.Weight < maxMatchValue {
			keepRecord = false
		}
		if keepRecord {
			summedMatchResults = append(summedMatchResults, prevTotal)
		}
	}

	for bI < len(i.CompareResults) {
		a := i.CompareResults[aI]
		b := i.CompareResults[bI]

		if a.areIdsEqual(b) {
			prevTotal.Weight += b.Weight
		} else {
			addRecord()
			prevTotal = b
		}

		aI++
		bI++
	}

	addRecord()

	i.CompareResults = summedMatchResults

}

func (i *IndexCompareResultList) getMaxWeight() int {
	var maxWeight int = 0
	for _, result := range i.CompareResults {
		if result.Weight > maxWeight {
			maxWeight = result.Weight
		}
	}

	return maxWeight
}

func (i *IndexCompareResultList) elevateWeight(lowerMaxWeight int) {
	for idx, _ := range i.CompareResults {
		i.CompareResults[idx].Weight += lowerMaxWeight
	}
}

func (i *IndexCompareResultList) trimUniqueMatches(uniqueMatchItems []CompareResult) {

	newLen := len(i.CompareResults) - len(uniqueMatchItems)
	trimmedList := make([]CompareResult, newLen)

	i.sort()
	sort.Sort(ByLeftRight(uniqueMatchItems))

	curI := 0
	sglI := 0
	trmI := 0
	var diff int

	for curI < len(i.CompareResults) {

		if sglI >= len(uniqueMatchItems) {
			diff = -1
		} else {
			diff = compareCompareResults(i.CompareResults[curI], uniqueMatchItems[sglI])
		}

		if diff < 0 {
			trimmedList[trmI] = i.CompareResults[curI]
			trmI++
			curI++

		} else if diff == 0 {
			curI++
			sglI++

		} else {
			sglI++

		}
	}

	if trmI != newLen {
		log.Error("Did not trim properly?? newLen: %d trmI: %d", newLen, trmI)
		log.Error("Did not trim properly?? len(i.CompareResults): %d len(uniqueMatchItems): %d", len(i.CompareResults), len(uniqueMatchItems))
	}

	i.CompareResults = trimmedList

}

func (i *IndexCompareResultList) ProcessMatches(splitMatch bool, maxMatchValue int) []CompareResult {

	if len(i.CompareResults) == 0 {
		return []CompareResult{}
	}

	i.sumMatchWeightValues(splitMatch, maxMatchValue)
	uniqueTopMatches := extractUniqueTopMatch(i)

	i.trimUniqueMatches(uniqueTopMatches)

	return uniqueTopMatches

}

func (i *IndexCompareResultList) MergeRemainingWeightType(remainingResults *IndexCompareResultList) {
	i.sumMatchWeightValues(false, 0)
	lowerMaxWeight := i.getMaxWeight()
	remainingResults.elevateWeight(lowerMaxWeight)

	i.CompareResults = append(i.CompareResults, remainingResults.CompareResults...)
	i.sort()
}