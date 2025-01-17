// @license
// Copyright 2021 Dynatrace LLC
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

package deploy

import (
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/api"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/coordinate"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/parameter"
)

type entityMap struct {
	resolvedEntities parameter.ResolvedEntities
	knownEntityNames map[string]map[string]struct{}
}

func newEntityMap(apis api.APIs) *entityMap {
	knownEntityNames := make(map[string]map[string]struct{})
	for _, a := range apis {
		knownEntityNames[a.ID] = make(map[string]struct{})
	}
	resolvedEntities := make(parameter.ResolvedEntities)
	return &entityMap{
		resolvedEntities: resolvedEntities,
		knownEntityNames: knownEntityNames,
	}
}

func (r *entityMap) put(coordinate coordinate.Coordinate, resolvedEntity parameter.ResolvedEntity) {
	// memorize resolved entity
	r.resolvedEntities[coordinate] = resolvedEntity

	// if entity was marked to be skipped we do not memorize the name of the entity
	// i.e., we do not care if the same name has already been used
	if resolvedEntity.Skip || resolvedEntity.EntityName == "" {
		return
	}

	// memorize the name of the resolved entity
	if _, found := r.knownEntityNames[coordinate.Type]; !found {
		r.knownEntityNames[coordinate.Type] = make(map[string]struct{})
	}
	r.knownEntityNames[coordinate.Type][resolvedEntity.EntityName] = struct{}{}
}

func (r *entityMap) get() parameter.ResolvedEntities {
	return r.resolvedEntities
}
func (r *entityMap) contains(entityType string, entityName string) bool {
	_, found := r.knownEntityNames[entityType][entityName]
	return found
}
