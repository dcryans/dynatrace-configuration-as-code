//go:build unit

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
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/client"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/parameter/value"
	"github.com/golang/mock/gomock"
	"strings"
	"testing"

	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/api"
	config "github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/coordinate"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/parameter"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/template"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/project/v2/topologysort"
	"github.com/google/uuid"
	"gotest.tools/assert"
)

var dashboardApi = api.API{ID: "dashboard", URLPath: "dashboard", DeprecatedBy: "dashboard-v2"}
var testApiMap = api.APIs{"dashboard": dashboardApi}

func TestDeployConfig(t *testing.T) {
	name := "test"
	owner := "hansi"
	ownerParameterName := "owner"
	timeout := 5
	timeoutParameterName := "timeout"
	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				Value: name,
			},
		},
		{
			Name: ownerParameterName,
			Parameter: &parameter.DummyParameter{
				Value: owner,
			},
		},
		{
			Name: timeoutParameterName,
			Parameter: &parameter.DummyParameter{
				Value: timeout,
			},
		},
	}

	client := &client.DummyClient{}
	conf := config.Config{
		Type:     config.ClassicApiType{Api: "dashboard"},
		Template: generateDummyTemplate(t),
		Coordinate: coordinate.Coordinate{
			Project:  "project1",
			Type:     "dashboard",
			ConfigId: "dashboard-1",
		},
		Environment: "development",
		Parameters:  toParameterMap(parameters),
		Skip:        false,
	}

	resolvedEntity, errors := deployConfig(client, testApiMap, newEntityMap(testApiMap), &conf)

	assert.Assert(t, len(errors) == 0, "there should be no errors (no errors: %d, %s)", len(errors), errors)
	assert.Equal(t, name, resolvedEntity.EntityName, "%s == %s")
	assert.Equal(t, conf.Coordinate, resolvedEntity.Coordinate)
	assert.Equal(t, name, resolvedEntity.Properties[config.NameParameter])
	assert.Equal(t, owner, resolvedEntity.Properties[ownerParameterName])
	assert.Equal(t, timeout, resolvedEntity.Properties[timeoutParameterName])
	assert.Equal(t, false, resolvedEntity.Skip)
}

func TestDeploySettingShouldFailCyclicParameterDependencies(t *testing.T) {
	ownerParameterName := "owner"
	configCoordinates := coordinate.Coordinate{}

	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				References: []parameter.ParameterReference{
					{
						Config:   configCoordinates,
						Property: ownerParameterName,
					},
				},
			},
		},
		{
			Name: ownerParameterName,
			Parameter: &parameter.DummyParameter{
				References: []parameter.ParameterReference{
					{
						Config:   configCoordinates,
						Property: config.NameParameter,
					},
				},
			},
		},
	}

	client := &client.DummyClient{}

	conf := &config.Config{
		Type:       config.ClassicApiType{},
		Template:   generateDummyTemplate(t),
		Parameters: toParameterMap(parameters),
	}
	_, errors := deploySetting(client, newEntityMap(testApiMap), conf)
	assert.Assert(t, len(errors) > 0, "there should be errors (no errors: %d)", len(errors))
}

func TestDeploySettingShouldFailRenderTemplate(t *testing.T) {
	client := &client.DummyClient{}

	conf := &config.Config{
		Type:     config.ClassicApiType{},
		Template: generateFaultyTemplate(t),
	}

	_, errors := deploySetting(client, newEntityMap(testApiMap), conf)
	assert.Assert(t, len(errors) > 0, "there should be errors (no errors: %d)", len(errors))
}

func TestDeploySettingShouldFailUpsert(t *testing.T) {
	name := "test"
	owner := "hansi"
	ownerParameterName := "owner"
	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				Value: name,
			},
		},
		{
			Name: ownerParameterName,
			Parameter: &parameter.DummyParameter{
				Value: owner,
			},
		},
		{
			Name: config.ScopeParameter,
			Parameter: &parameter.DummyParameter{
				Value: "something",
			},
		},
	}

	c := client.NewMockSettingsClient(gomock.NewController(t))
	c.EXPECT().UpsertSettings(gomock.Any()).Return(client.DynatraceEntity{}, fmt.Errorf("upsert failed"))

	conf := &config.Config{
		Type:       config.SettingsType{},
		Template:   generateDummyTemplate(t),
		Parameters: toParameterMap(parameters),
	}
	_, errors := deploySetting(c, newEntityMap(testApiMap), conf)
	assert.Assert(t, len(errors) > 0, "there should be errors (no errors: %d)", len(errors))
}

func TestDeploySetting(t *testing.T) {
	parameters := []topologysort.ParameterWithName{
		{
			Name: "franz",
			Parameter: &parameter.DummyParameter{
				Value: "foo",
			},
		},
		{
			Name: "hansi",
			Parameter: &parameter.DummyParameter{
				Value: "bar",
			},
		},
		{
			Name: config.ScopeParameter,
			Parameter: &parameter.DummyParameter{
				Value: "something",
			},
		},
	}

	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().UpsertSettings(gomock.Any()).Times(1).Return(client.DynatraceEntity{
		Id:   "vu9U3hXa3q0AAAABABlidWlsdGluOMmE1NGMxvu9U3hXa3q0",
		Name: "vu9U3hXa3q0AAAABABlidWlsdGluOMmE1NGMxvu9U3hXa3q0",
	}, nil)

	conf := &config.Config{
		Type:       config.SettingsType{},
		Template:   generateDummyTemplate(t),
		Parameters: toParameterMap(parameters),
	}
	_, errors := deploySetting(c, newEntityMap(testApiMap), conf)
	assert.Assert(t, len(errors) == 0, "there should be no errors (no errors: %d, %s)", len(errors), errors)
}

func TestDeployedSettingGetsNameFromConfig(t *testing.T) {
	cfgName := "THE CONFIG NAME"

	parameters := []topologysort.ParameterWithName{
		{
			Name: "franz",
			Parameter: &parameter.DummyParameter{
				Value: "foo",
			},
		},
		{
			Name: "hansi",
			Parameter: &parameter.DummyParameter{
				Value: "bar",
			},
		},
		{
			Name: config.ScopeParameter,
			Parameter: &parameter.DummyParameter{
				Value: "something",
			},
		},
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				Value: cfgName,
			},
		},
	}

	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().UpsertSettings(gomock.Any()).Times(1).Return(client.DynatraceEntity{
		Id:   "vu9U3hXa3q0AAAABABlidWlsdGluOMmE1NGMxvu9U3hXa3q0",
		Name: "vu9U3hXa3q0AAAABABlidWlsdGluOMmE1NGMxvu9U3hXa3q0",
	}, nil)

	conf := &config.Config{
		Type:       config.SettingsType{},
		Template:   generateDummyTemplate(t),
		Parameters: toParameterMap(parameters),
	}
	res, errors := deploySetting(c, newEntityMap(testApiMap), conf)
	assert.Equal(t, res.EntityName, cfgName, "expected resolved name to match configuration name")
	assert.Assert(t, len(errors) == 0, "there should be no errors (no errors: %d, %s)", len(errors), errors)
}

func TestSettingsNameExtractionDoesNotFailIfCfgNameBecomesOptional(t *testing.T) {
	parametersWithoutName := []topologysort.ParameterWithName{
		{
			Name: "franz",
			Parameter: &parameter.DummyParameter{
				Value: "foo",
			},
		},
		{
			Name: "hansi",
			Parameter: &parameter.DummyParameter{
				Value: "bar",
			},
		},
		{
			Name: config.ScopeParameter,
			Parameter: &parameter.DummyParameter{
				Value: "something",
			},
		},
	}

	objectId := "vu9U3hXa3q0AAAABABlidWlsdGluOMmE1NGMxvu9U3hXa3q0"

	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().UpsertSettings(gomock.Any()).Times(1).Return(client.DynatraceEntity{
		Id:   objectId,
		Name: objectId,
	}, nil)

	conf := &config.Config{
		Type:       config.SettingsType{},
		Template:   generateDummyTemplate(t),
		Parameters: toParameterMap(parametersWithoutName),
	}
	res, errors := deploySetting(c, newEntityMap(testApiMap), conf)
	assert.Assert(t, strings.Contains(res.EntityName, objectId), "expected resolved name to contain objectID if name is not configured")
	assert.Assert(t, len(errors) == 0, "there should be no errors (no errors: %d, %s)", len(errors), errors)
}

func TestDeployConfigShouldFailOnAnAlreadyKnownEntityName(t *testing.T) {
	name := "test"
	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				Value: name,
			},
		},
	}

	client := &client.DummyClient{}
	conf := config.Config{
		Type:     config.ClassicApiType{Api: "dashboard"},
		Template: generateDummyTemplate(t),
		Coordinate: coordinate.Coordinate{
			Project:  "project1",
			Type:     "dashboard",
			ConfigId: "dashboard-1",
		},
		Environment: "development",
		Parameters:  toParameterMap(parameters),
		Skip:        false,
	}
	entityMap := newEntityMap(testApiMap)
	entityMap.put(coordinate.Coordinate{Type: "dashboard"}, parameter.ResolvedEntity{EntityName: name})
	_, errors := deployConfig(client, testApiMap, entityMap, &conf)

	assert.Assert(t, len(errors) > 0, "there should be errors (no errors: %d)", len(errors))
}

func TestDeployConfigShouldFailCyclicParameterDependencies(t *testing.T) {
	ownerParameterName := "owner"
	configCoordinates := coordinate.Coordinate{
		Project:  "project1",
		Type:     "dashboard",
		ConfigId: "dashboard-1",
	}

	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				References: []parameter.ParameterReference{
					{
						Config:   configCoordinates,
						Property: ownerParameterName,
					},
				},
			},
		},
		{
			Name: ownerParameterName,
			Parameter: &parameter.DummyParameter{
				References: []parameter.ParameterReference{
					{
						Config:   configCoordinates,
						Property: config.NameParameter,
					},
				},
			},
		},
	}

	client := &client.DummyClient{}
	conf := config.Config{
		Type:     config.ClassicApiType{Api: "dashboard"},
		Template: generateDummyTemplate(t),
		Coordinate: coordinate.Coordinate{
			Project:  "project1",
			Type:     "dashboard",
			ConfigId: "dashboard-1",
		},
		Environment: "development",
		Parameters:  toParameterMap(parameters),
		Skip:        false,
	}

	_, errors := deployConfig(client, testApiMap, newEntityMap(testApiMap), &conf)
	assert.Assert(t, len(errors) > 0, "there should be errors (no errors: %d)", len(errors))
}

func TestDeployConfigShouldFailOnMissingNameParameter(t *testing.T) {
	parameters := []topologysort.ParameterWithName{}

	client := &client.DummyClient{}
	conf := config.Config{
		Type:     config.ClassicApiType{Api: "dashboard"},
		Template: generateDummyTemplate(t),
		Coordinate: coordinate.Coordinate{
			Project:  "project1",
			Type:     "dashboard",
			ConfigId: "dashboard-1",
		},
		Environment: "development",
		Parameters:  toParameterMap(parameters),
		Skip:        false,
	}

	_, errors := deployConfig(client, testApiMap, newEntityMap(testApiMap), &conf)
	assert.Assert(t, len(errors) > 0, "there should be errors (no errors: %d)", len(errors))
}

func TestDeployConfigShouldFailOnReferenceOnUnknownConfig(t *testing.T) {
	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				References: []parameter.ParameterReference{
					{
						Config: coordinate.Coordinate{
							Project:  "project2",
							Type:     "dashboard",
							ConfigId: "dashboard",
						},
						Property: "managementZoneId",
					},
				},
			},
		},
	}

	client := &client.DummyClient{}
	conf := config.Config{
		Type:     config.ClassicApiType{Api: "dashboard"},
		Template: generateDummyTemplate(t),
		Coordinate: coordinate.Coordinate{
			Project:  "project1",
			Type:     "dashboard",
			ConfigId: "dashboard-1",
		},
		Environment: "development",
		Parameters:  toParameterMap(parameters),
		Skip:        false,
	}

	_, errors := deployConfig(client, testApiMap, newEntityMap(testApiMap), &conf)
	assert.Assert(t, len(errors) > 0, "there should be errors (no errors: %d)", len(errors))
}

func TestDeployConfigShouldFailOnReferenceOnSkipConfig(t *testing.T) {
	referenceCoordinates := coordinate.Coordinate{
		Project:  "project2",
		Type:     "dashboard",
		ConfigId: "dashboard",
	}

	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				References: []parameter.ParameterReference{
					{
						Config:   referenceCoordinates,
						Property: "managementZoneId",
					},
				},
			},
		},
	}

	client := &client.DummyClient{}
	conf := config.Config{
		Type:     config.ClassicApiType{Api: "dashboard"},
		Template: generateDummyTemplate(t),
		Coordinate: coordinate.Coordinate{
			Project:  "project1",
			Type:     "dashboard",
			ConfigId: "dashboard-1",
		},
		Environment: "development",
		Parameters:  toParameterMap(parameters),
		Skip:        false,
	}

	_, errors := deployConfig(client, testApiMap, newEntityMap(testApiMap), &conf)
	assert.Assert(t, len(errors) > 0, "there should be errors (no errors: %d)", len(errors))
}

func TestDeployConfigsWithNoConfigs(t *testing.T) {
	client := &client.DummyClient{}
	var apis api.APIs
	var sortedConfigs []config.Config

	errors := DeployConfigs(client, apis, sortedConfigs, DeployConfigsOptions{})
	assert.Assert(t, len(errors) == 0, "there should be no errors (errors: %s)", errors)
}

func TestDeployConfigsWithOneConfigToSkip(t *testing.T) {
	client := &client.DummyClient{}
	var apis api.APIs
	sortedConfigs := []config.Config{
		{Skip: true},
	}
	errors := DeployConfigs(client, apis, sortedConfigs, DeployConfigsOptions{})
	assert.Assert(t, len(errors) == 0, "there should be no errors (errors: %s)", errors)
}

func TestDeployConfigsTargetingSettings(t *testing.T) {
	c := client.NewMockClient(gomock.NewController(t))
	var apis api.APIs
	sortedConfigs := []config.Config{
		{
			Template: generateDummyTemplate(t),
			Coordinate: coordinate.Coordinate{
				Project:  "some project",
				Type:     "schema",
				ConfigId: "some setting",
			},
			Type: config.SettingsType{
				SchemaId:      "schema",
				SchemaVersion: "schemaversion",
			},
			Parameters: config.Parameters{
				config.ScopeParameter: &value.ValueParameter{Value: "tenant"},
			},
		},
	}
	//client.EXPECT().ListSettings(gomock.Any(), gomock.Any()).Times(1).Return([]rest.DownloadSettingsObject{{ExternalId: "externalId"}}, nil)
	c.EXPECT().UpsertSettings(gomock.Any()).Times(1).Return(client.DynatraceEntity{
		Id:   "42",
		Name: "Super Special Settings Object",
	}, nil)
	errors := DeployConfigs(c, apis, sortedConfigs, DeployConfigsOptions{})
	assert.Assert(t, len(errors) == 0, "there should be no errors (errors: %s)", errors)
}

func TestDeployConfigsTargetingClassicConfigUnique(t *testing.T) {
	theConfigName := "theConfigName"
	theApiName := "theApiName"

	theApi := api.API{ID: theApiName, URLPath: "path"}

	client := client.NewMockClient(gomock.NewController(t))
	client.EXPECT().UpsertConfigByName(gomock.Any(), theConfigName, gomock.Any()).Times(1)

	apis := api.APIs{theApiName: theApi}
	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				Value: theConfigName,
			},
		},
	}
	sortedConfigs := []config.Config{
		{
			Parameters: toParameterMap(parameters),
			Coordinate: coordinate.Coordinate{Type: theApiName},
			Template:   generateDummyTemplate(t),
			Type: config.ClassicApiType{
				Api: theApiName,
			},
		},
	}

	errors := DeployConfigs(client, apis, sortedConfigs, DeployConfigsOptions{})
	assert.Assert(t, len(errors) == 0, "there should be no errors (errors: %s)", errors)
}

func TestDeployConfigsTargetingClassicConfigNonUniqueWithExistingCfgsOfSameName(t *testing.T) {
	theConfigName := "theConfigName"
	theApiName := "theApiName"

	theApi := api.API{ID: theApiName, URLPath: "path", NonUniqueName: true}

	client := client.NewMockClient(gomock.NewController(t))
	client.EXPECT().UpsertConfigByNonUniqueNameAndId(gomock.Any(), gomock.Any(), theConfigName, gomock.Any())

	apis := api.APIs{theApiName: theApi}
	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				Value: theConfigName,
			},
		},
	}
	sortedConfigs := []config.Config{
		{
			Parameters: toParameterMap(parameters),
			Coordinate: coordinate.Coordinate{Type: theApiName},
			Template:   generateDummyTemplate(t),
			Type: config.ClassicApiType{
				Api: theApiName,
			},
		},
	}

	errors := DeployConfigs(client, apis, sortedConfigs, DeployConfigsOptions{})
	assert.Assert(t, len(errors) == 0, "there should be no errors (errors: %s)", errors)
}

func TestDeployConfigsNoApi(t *testing.T) {
	theConfigName := "theConfigName"
	theApiName := "theApiName"

	client := client.NewMockClient(gomock.NewController(t))

	apis := api.APIs{}
	parameters := []topologysort.ParameterWithName{
		{
			Name: config.NameParameter,
			Parameter: &parameter.DummyParameter{
				Value: theConfigName,
			},
		},
	}
	sortedConfigs := []config.Config{
		{
			Parameters: toParameterMap(parameters),
			Coordinate: coordinate.Coordinate{Type: theApiName},
			Template:   generateDummyTemplate(t),
			Type: config.ClassicApiType{
				Api: theApiName,
			},
		},
		{
			Parameters: toParameterMap(parameters),
			Coordinate: coordinate.Coordinate{Type: theApiName},
			Template:   generateDummyTemplate(t),
			Type: config.ClassicApiType{
				Api: theApiName,
			},
		},
	}

	t.Run("missing api - continue on error", func(t *testing.T) {
		errors := DeployConfigs(client, apis, sortedConfigs, DeployConfigsOptions{ContinueOnErr: true})
		assert.Equal(t, 2, len(errors), fmt.Sprintf("Expected 2 errors, but just got %d", len(errors)))
	})

	t.Run("missing api - stop on error", func(t *testing.T) {
		errors := DeployConfigs(client, apis, sortedConfigs, DeployConfigsOptions{})
		assert.Equal(t, 1, len(errors), fmt.Sprintf("Expected 1 error, but just got %d", len(errors)))
	})
	// test continue on error

}

func TestDeployConfigsWithDeploymentErrors(t *testing.T) {
	theApiName := "theApiName"
	theApi := api.API{ID: theApiName, URLPath: "path"}
	apis := api.APIs{theApiName: theApi}
	sortedConfigs := []config.Config{
		{
			Parameters: toParameterMap([]topologysort.ParameterWithName{}), // missing name parameter leads to deployment failure
			Coordinate: coordinate.Coordinate{Type: theApiName},
			Template:   generateDummyTemplate(t),
			Type: config.ClassicApiType{
				Api: theApiName,
			},
		},
		{
			Parameters: toParameterMap([]topologysort.ParameterWithName{}), // missing name parameter leads to deployment failure
			Coordinate: coordinate.Coordinate{Type: theApiName},
			Template:   generateDummyTemplate(t),
			Type: config.ClassicApiType{
				Api: theApiName,
			},
		},
	}

	t.Run("deployment error - stop on error", func(t *testing.T) {
		errors := DeployConfigs(&client.DummyClient{}, apis, sortedConfigs, DeployConfigsOptions{})
		assert.Equal(t, 1, len(errors), fmt.Sprintf("Expected 1 error, but just got %d", len(errors)))
	})

	t.Run("deployment error - stop on error", func(t *testing.T) {
		errors := DeployConfigs(&client.DummyClient{}, apis, sortedConfigs, DeployConfigsOptions{ContinueOnErr: true})
		assert.Equal(t, 2, len(errors), fmt.Sprintf("Expected 1 error, but just got %d", len(errors)))
	})

}

func toParameterMap(params []topologysort.ParameterWithName) map[string]parameter.Parameter {
	result := make(map[string]parameter.Parameter)

	for _, p := range params {
		result[p.Name] = p.Parameter
	}

	return result
}

func generateDummyTemplate(t *testing.T) template.Template {
	uuid, err := uuid.NewUUID()
	assert.NilError(t, err)
	templ := template.CreateTemplateFromString("deploy_test-"+uuid.String(), "{}")
	return templ
}

func generateFaultyTemplate(t *testing.T) template.Template {
	uuid, err := uuid.NewUUID()
	assert.NilError(t, err)
	templ := template.CreateTemplateFromString("deploy_test-"+uuid.String(), "{")
	return templ
}
