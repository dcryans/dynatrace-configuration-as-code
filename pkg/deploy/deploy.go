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
	"fmt"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/api"
	config "github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/config/v2"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/config/v2/coordinate"
	configErrors "github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/config/v2/errors"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/config/v2/parameter"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/rest"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/util"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/util/log"
)

type configDeployError struct {
	Config             coordinate.Coordinate
	EnvironmentDetails configErrors.EnvironmentDetails
	Reason             string
}

func newConfigDeployError(conf *config.Config, reason string) configDeployError {
	return configDeployError{
		Config: conf.Coordinate,
		EnvironmentDetails: configErrors.EnvironmentDetails{
			Group:       conf.Group,
			Environment: conf.Environment,
		},
		Reason: reason,
	}
}

func (e configDeployError) Coordinates() coordinate.Coordinate {
	return e.Config
}

func (e configDeployError) LocationDetails() configErrors.EnvironmentDetails {
	return e.EnvironmentDetails
}

func (e configDeployError) Error() string {
	return e.Reason
}

type ParameterReferenceError struct {
	Config             coordinate.Coordinate
	EnvironmentDetails configErrors.EnvironmentDetails
	Parameter          string
	Reference          parameter.ParameterReference
	Reason             string
}

func newParameterReferenceError(coord coordinate.Coordinate, group string, env string,
	param string, ref parameter.ParameterReference, reason string) ParameterReferenceError {
	return ParameterReferenceError{
		Config: coord,
		EnvironmentDetails: configErrors.EnvironmentDetails{
			Group:       group,
			Environment: env,
		},
		Parameter: param,
		Reference: ref,
		Reason:    reason,
	}
}

func (e ParameterReferenceError) Coordinates() coordinate.Coordinate {
	return e.Config
}

func (e ParameterReferenceError) LocationDetails() configErrors.EnvironmentDetails {
	return e.EnvironmentDetails
}

func (e ParameterReferenceError) Error() string {
	return fmt.Sprintf("parameter `%s` cannot reference `%s`: %s",
		e.Parameter, e.Reference, e.Reason)
}

var (
	_ configErrors.DetailedConfigError = (*configDeployError)(nil)
	_ configErrors.DetailedConfigError = (*ParameterReferenceError)(nil)
)

// DeployConfigs deploys the given configs with the given apis via the given client
// NOTE: the given configs need to be sorted, otherwise deployment will
// probably fail, as references cannot be resolved
func DeployConfigs(client rest.DynatraceClient, apis api.ApiMap,
	sortedConfigs []config.Config, continueOnError, dryRun bool) []error {

	entityMap := NewEntityMap(apis)
	var errors []error

	for _, c := range sortedConfigs {
		c := c // to avoid implicit memory aliasing (gosec G601)

		if c.Skip {
			entityMap.PutResolved(c.Coordinate, parameter.ResolvedEntity{
				EntityName: c.Coordinate.ConfigId,
				Coordinate: c.Coordinate,
				Properties: parameter.Properties{},
				Skip:       true,
			})
			// if the config is skip we do not care if the same name
			// has already been used
			continue
		}

		var entity parameter.ResolvedEntity
		var deploymentErrors []error

		if c.Type.IsSettings() {
			entity, deploymentErrors = deploySetting(client, entityMap, &c)
		} else {
			entity, deploymentErrors = deployConfig(client, apis, entityMap, &c)
		}

		if deploymentErrors != nil {
			errors = append(errors, deploymentErrors...)

			if continueOnError || dryRun {
				continue
			} else {
				return errors
			}
		}
		entityMap.PutResolved(entity.Coordinate, entity)
	}

	return errors
}

func deployConfig(client rest.ConfigClient, apis api.ApiMap, entityMap *EntityMap, conf *config.Config) (parameter.ResolvedEntity, []error) {

	apiToDeploy := apis[conf.Coordinate.Type]
	if apiToDeploy == nil {
		return parameter.ResolvedEntity{}, []error{fmt.Errorf("unknown api `%s`. this is most likely a bug", conf.Type.Api)}
	}

	properties, errors := resolveProperties(conf, entityMap.Resolved())
	if len(errors) > 0 {
		return parameter.ResolvedEntity{}, errors
	}

	configName, err := extractConfigName(conf, properties)
	if err != nil {
		errors = append(errors, err)
	} else {
		if entityMap.Known(apiToDeploy.GetId(), configName) && !apiToDeploy.IsNonUniqueNameApi() {
			errors = append(errors, newConfigDeployError(conf, fmt.Sprintf("duplicated config name `%s`", configName)))
		}
	}
	if len(errors) > 0 {
		return parameter.ResolvedEntity{}, errors
	}

	renderedConfig, err := conf.Render(properties)
	if err != nil {
		return parameter.ResolvedEntity{}, []error{err}
	}

	if apiToDeploy.IsDeprecatedApi() {
		log.Warn("API for \"%s\" is deprecated! Please consider migrating to \"%s\"!", apiToDeploy.GetId(), apiToDeploy.IsDeprecatedBy())
	}

	var entity api.DynatraceEntity
	if apiToDeploy.IsNonUniqueNameApi() {
		configId := conf.Coordinate.ConfigId
		projectId := conf.Coordinate.Project

		entityUuid := configId

		isUuidOrMeId := util.IsUuid(entityUuid) || util.IsMeId(entityUuid)
		if !isUuidOrMeId {
			entityUuid, err = util.GenerateUuidFromConfigId(projectId, configId)
			if err != nil {
				return parameter.ResolvedEntity{}, []error{newConfigDeployError(conf, err.Error())}
			}
		}

		entity, err = client.UpsertByEntityId(apiToDeploy, entityUuid, configName, []byte(renderedConfig))
	} else {
		entity, err = client.UpsertByName(apiToDeploy, configName, []byte(renderedConfig))
	}

	if err != nil {
		return parameter.ResolvedEntity{}, []error{newConfigDeployError(conf, err.Error())}
	}

	properties[config.IdParameter] = entity.Id
	properties[config.NameParameter] = entity.Name

	return parameter.ResolvedEntity{
		EntityName: entity.Name,
		Coordinate: conf.Coordinate,
		Properties: properties,
		Skip:       false,
	}, nil
}

func deploySetting(client rest.SettingsClient, entityMap *EntityMap, c *config.Config) (parameter.ResolvedEntity, []error) {

	settings, err := client.ListKnownSettings([]string{c.Type.Schema})
	if err != nil {
		// continue & dry run missing
		return parameter.ResolvedEntity{}, []error{fmt.Errorf("failed to list known settings: %w", err)}
	}

	properties, errors := resolveProperties(c, entityMap.Resolved())
	if len(errors) > 0 {
		return parameter.ResolvedEntity{}, errors
	}

	renderedConfig, err := c.Render(properties)
	if err != nil {
		return parameter.ResolvedEntity{}, []error{err}
	}

	e, err := client.Upsert(settings, rest.SettingsObject{
		Id:            c.Coordinate.ConfigId,
		Schema:        c.Type.Schema,
		SchemaVersion: c.Type.SchemaVersion,
		Scope:         c.Type.Scope,
		Content:       []byte(renderedConfig),
	})
	if err != nil {
		return parameter.ResolvedEntity{}, []error{newConfigDeployError(c, err.Error())}
	}

	properties[config.IdParameter] = e.Id
	properties[config.NameParameter] = e.Name

	return parameter.ResolvedEntity{
		EntityName: e.Name,
		Coordinate: c.Coordinate,
		Properties: properties,
		Skip:       false,
	}, nil

}
