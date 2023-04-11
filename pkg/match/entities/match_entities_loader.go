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

package entities

import (
	"encoding/json"
	"path/filepath"

	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match"
	"github.com/spf13/afero"
)

func LoadMatches(fs afero.Fs, matchParameters match.MatchParameters) (MatchOutputPerType, error) {

	filesInFolder, err := afero.ReadDir(fs, matchParameters.EntitiesMatchDir)
	if err != nil {
		return nil, err
	}

	matchOutputPerType := MatchOutputPerType{}

	for _, file := range filesInFolder {
		filename := file.Name()

		if file.IsDir() {
			continue
		}

		data, err := afero.ReadFile(fs, filepath.Join(matchParameters.EntitiesMatchDir, filename))
		if err != nil {
			return nil, err
		}

		matchOutputType := MatchOutputType{}
		err = json.Unmarshal(data, &matchOutputType)
		if err != nil {
			return nil, err
		}

		matchOutputPerType[matchOutputType.Type] = matchOutputType

	}

	return matchOutputPerType, nil

}
