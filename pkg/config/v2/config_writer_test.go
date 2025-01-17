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

//go:build unit

package v2

import (
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/errutils"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/log"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/testutils"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/coordinate"
	envParam "github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/parameter/environment"
	refParam "github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/parameter/reference"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/template"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"testing"

	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/parameter"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2/parameter/value"
	"gotest.tools/assert"
)

func TestExtractCommonBase(t *testing.T) {
	configName := "test-config-1"
	group := "development"
	template := "test.json"

	param1Name := "config number"
	param1Value := "12"

	param2Name := "dashboardId"
	param2Value := []interface{}{"projectA", "dashboard", "important", "id"}

	param3Name := "dashboardId2"
	param3Value := map[interface{}]interface{}{
		"type":     "reference",
		"project":  "projectA",
		"api":      "dashboard",
		"config":   "test",
		"property": "id",
	}

	parameterNotSharedName := "not-shared"

	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name:             param1Value,
					param2Name:             param2Value,
					param3Name:             param3Value,
					parameterNotSharedName: "not-shared",
				},
				Template: template,
				Skip:     nil,
			},
			group:       group,
			environment: "test",
		},
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name:             param1Value,
					param2Name:             param2Value,
					param3Name:             param3Value,
					parameterNotSharedName: 12,
				},
				Template: template,
				Skip:     nil,
			},
			group:       group,
			environment: "test1",
		},
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name:             param1Value,
					param2Name:             param2Value,
					param3Name:             param3Value,
					parameterNotSharedName: 25,
				},
				Template: template,
				Skip:     nil,
			},
			group:       group,
			environment: "test2",
		},
	}

	base, rest := extractCommonBase(configs)

	assert.Assert(t, base != nil, "there should be a common base")

	assert.Assert(t, base.Name == configName, "name should be `%s`, but was `%s`", configName, base.Name)
	assert.Assert(t, base.Template == template, "template should be `%s`, but was `%s`", template, base.Template)
	assert.Assert(t, base.Skip == nil, "skip should be nil: %v", base.Skip)
	assert.Assert(t, len(base.Parameters) == 3, "there should be 3 parameter overrides, but there were `%d`",
		len(base.Parameters))

	for _, n := range []string{param1Name, param2Name, param3Name} {
		param := base.Parameters[n]
		assert.Assert(t, param != nil, "`%s` should be present in base", n)
	}

	assert.Assert(t, len(rest) == 3, "there should be `3` overrides, but there were `%d`", len(rest))

	for _, r := range rest {
		for _, n := range []string{param1Name, param2Name, param3Name} {
			param := r.Parameters[n]
			assert.Assert(t, param == nil, "`%s` should not be present in override for `%s`", n, r.environment)
		}
	}
}

func TestExtractCommonBaseForEnvVarSkipsWithEqualValues(t *testing.T) {
	configName := "test-config-1"
	group := "development"
	template := "test.json"

	param1Name := "config number"
	param1Value := "12"

	param2Name := "dashboardId"
	param2Value := []interface{}{"projectA", "dashboard", "important", "id"}

	param3Name := "dashboardId2"
	param3Value := map[interface{}]interface{}{
		"type":     "reference",
		"project":  "projectA",
		"api":      "dashboard",
		"config":   "test",
		"property": "id",
	}

	parameterNotSharedName := "not-shared"

	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name:             param1Value,
					param2Name:             param2Value,
					param3Name:             param3Value,
					parameterNotSharedName: 12,
				},
				Template: template,
				Skip: map[any]any{
					"type": "environment",
					"name": "A",
				},
			},
			group:       group,
			environment: "test",
		},
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name:             param1Value,
					param2Name:             param2Value,
					param3Name:             param3Value,
					parameterNotSharedName: 13,
				},
				Template: template,
				Skip: map[any]any{
					"type": "environment",
					"name": "A",
				},
			},
			group:       group,
			environment: "test1",
		},
	}

	base, rest := extractCommonBase(configs)

	assert.Assert(t, base != nil, "there should be a common base")

	assert.Assert(t, base.Name == configName, "name should be `%s`, but was `%s`", configName, base.Name)
	assert.Assert(t, base.Template == template, "template should be `%s`, but was `%s`", template, base.Template)
	assert.Assert(t, base.Skip != nil, "skip should not be nil")
	assert.Assert(t, len(base.Parameters) == 3, "there should be 3 base-parameters, but there were `%d`", len(base.Parameters))

	for _, n := range []string{param1Name, param2Name, param3Name} {
		param := base.Parameters[n]
		assert.Assert(t, param != nil, "`%s` should be present in base", n)
	}

	assert.Assert(t, base.Skip != nil, "skip should be in the base")

	assert.DeepEqual(t, base.Skip, map[any]any{
		"type": "environment",
		"name": "A",
	})

	assert.Assert(t, len(rest) == 2, "there should be `2` overrides, but there were `%d`", len(rest))

	for _, r := range rest {
		for _, n := range []string{param1Name, param2Name, param3Name} {
			param := r.Parameters[n]
			assert.Assert(t, param == nil, "`%s` should not be present in override for `%s`", n, r.environment)
		}
	}
}

func TestExtractCommonBaseForEnvVarSkipsWithDifferentValues(t *testing.T) {
	configName := "test-config-1"
	group := "development"
	template := "test.json"

	param1Name := "config number"
	param1Value := "12"

	param2Name := "dashboardId"
	param2Value := []interface{}{"projectA", "dashboard", "important", "id"}

	param3Name := "dashboardId2"
	param3Value := map[interface{}]interface{}{
		"type":     "reference",
		"project":  "projectA",
		"api":      "dashboard",
		"config":   "test",
		"property": "id",
	}

	skipA := map[any]any{
		"type": "environment",
		"name": "A",
	}
	skipB := map[any]any{
		"type": "environment",
		"name": "B",
	}
	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name: param1Value,
					param2Name: param2Value,
					param3Name: param3Value,
				},
				Template: template,
				Skip:     skipA,
			},
			group:       group,
			environment: "test",
		},
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name: param1Value,
					param2Name: param2Value,
					param3Name: param3Value,
				},
				Template: template,
				Skip:     skipB,
			},
			group:       group,
			environment: "test1",
		},
	}

	base, rest := extractCommonBase(configs)

	assert.Assert(t, base != nil, "there should be a common base")

	assert.Assert(t, base.Name == configName, "name should be `%s`, but was `%s`", configName, base.Name)
	assert.Assert(t, base.Template == template, "template should be `%s`, but was `%s`", template, base.Template)
	assert.Assert(t, base.Skip == nil, "base skip should be nil")
	assert.Assert(t, len(base.Parameters) == 3, "there should be 3 base-parameters, but there were `%d`", len(base.Parameters))

	for _, n := range []string{param1Name, param2Name, param3Name} {
		param := base.Parameters[n]
		assert.Assert(t, param != nil, "`%s` should be present in base", n)
	}

	assert.Assert(t, len(rest) == 2, "there should be `2` overrides, but there were `%d`", len(rest))

	for _, r := range rest {
		for _, n := range []string{param1Name, param2Name, param3Name} {
			param := r.Parameters[n]
			assert.Assert(t, param == nil, "`%s` should not be present in override for `%s`", n, r.environment)
		}
	}

	assert.DeepEqual(t, rest[0].Skip, skipA)
	assert.DeepEqual(t, rest[1].Skip, skipB)
}

func TestExtractCommonBaseT(t *testing.T) {
	configName := "test-config-1"
	group := "development"
	template := "test.json"

	param1Name := "config number"
	param1Value := "12"

	param2Name := "dashboardId"
	param2Value := []interface{}{"projectA", "dashboard", "important", "id"}

	param3Name := "dashboardId2"
	param3Value := map[interface{}]interface{}{
		"type":     "reference",
		"project":  "projectA",
		"api":      "dashboard",
		"config":   "test",
		"property": "id",
	}

	parameterNotSharedName := "not-shared"

	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name:             param1Value,
					param2Name:             param2Value,
					param3Name:             param3Value,
					parameterNotSharedName: "not-shared",
				},
				Template: template,
				Skip:     nil,
			},
			group:       group,
			environment: "test",
		},
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name:             param1Value,
					param2Name:             param2Value,
					param3Name:             param3Value,
					parameterNotSharedName: 12,
				},
				Template: template,
				Skip:     nil,
			},
			group:       group,
			environment: "test1",
		},
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name:             param1Value,
					param2Name:             param2Value,
					param3Name:             param3Value,
					parameterNotSharedName: 25,
				},
				Template: template,
				Skip:     true,
			},
			group:       group,
			environment: "test2",
		},
	}

	base, rest := extractCommonBase(configs)

	assert.Assert(t, base != nil, "there should be a common base")

	assert.Assert(t, base.Name == configName, "name should be `%s`, but was `%s`", configName, base.Name)
	assert.Assert(t, base.Template == template, "template should be `%s`, but was `%s`", template, base.Template)
	assert.Assert(t, base.Skip == nil, "skip should be nil: %v", base.Skip)
	assert.Assert(t, len(base.Parameters) == 3, "there should be 3 parameter overrides, but there were `%d`",
		len(base.Parameters))

	for _, n := range []string{param1Name, param2Name, param3Name} {
		param := base.Parameters[n]
		assert.Assert(t, param != nil, "`%s` should be present in base", n)
	}

	assert.Assert(t, len(rest) == 3, "there should be `3` overrides, but there were `%d`", len(rest))

	for _, r := range rest {
		for _, n := range []string{param1Name, param2Name, param3Name} {
			param := r.Parameters[n]
			assert.Assert(t, param == nil, "`%s` should not be present in override for `%s`", n, r.environment)
		}
	}
}

func TestExtractCommonBaseWithJustSkipDifferent(t *testing.T) {
	configName := "test-config-1"
	group := "development"
	template := "test.json"

	param1Name := "config number"
	param1Value := "12"

	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name: param1Value,
				},
				Template: template,
				Skip:     nil,
			},
			group:       group,
			environment: "test",
		},
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name: param1Value,
				},
				Template: template,
				Skip:     true,
			},
			group:       group,
			environment: "test1",
		},
		{
			configDefinition: configDefinition{
				Name: configName,
				Parameters: map[string]configParameter{
					param1Name: param1Value,
				},
				Template: template,
				Skip:     nil,
			},
			group:       group,
			environment: "test2",
		},
	}

	base, rest := extractCommonBase(configs)

	assert.Assert(t, base != nil, "there should be a common base")

	assert.Assert(t, base.Name == configName, "name should be `%s`, but was `%s`", configName, base.Name)
	assert.Assert(t, base.Template == template, "template should be `%s`, but was `%s`", template, base.Template)
	assert.Assert(t, base.Skip == nil, "skip should be nil: %v", base.Skip)
	assert.Assert(t, len(base.Parameters) == 1, "there should be 1 parameter overrides, but there were `%d`",
		len(base.Parameters))

	assert.Assert(t, base.Parameters[param1Name] != nil, "`%s` should be present in base", param1Name)

	assert.Assert(t, len(rest) == 3, "there should be `3` overrides, but there were `%d`", len(rest))

	for _, r := range rest {
		assert.Assert(t, r.Parameters[param1Name] == nil, "`%s` should not be present in override for `%s`",
			param1Name, r.environment)
	}
}

func TestToParameterDefinition(t *testing.T) {
	paramName := "test-param-1"
	paramValue := "hello"

	context := detailedSerializerContext{
		serializerContext: &serializerContext{
			WriterContext: &WriterContext{
				ParametersSerde: map[string]parameter.ParameterSerDe{
					parameter.DummyParameterType: {
						Serializer: func(c parameter.ParameterWriterContext) (map[string]interface{}, error) {
							return map[string]interface{}{
								"Value": c.Parameter.(*parameter.DummyParameter).Value,
							}, nil
						},
					},
				},
			},
		},
	}

	result, err := toParameterDefinition(&context, paramName, &parameter.DummyParameter{
		Value: paramValue,
	})

	assert.NilError(t, err, "to parameter definiton should return no error, but was `%s`", err)
	assert.Assert(t, result != nil, "result should not be nil")

	resultMap, ok := result.(map[string]interface{})

	assert.Assert(t, ok, "result should be a map")
	assert.Assert(t, resultMap["Value"] == "hello", "result should have key `Value` with value `%s`, but was `%s`",
		paramValue, resultMap["Value"])
}

func TestToParameterDefinitionShouldDoSpecialParameterDefinitionIfActivatedAndSupported(t *testing.T) {
	paramName := "test-param-1"
	paramValue := "hello"

	context := detailedSerializerContext{
		serializerContext: &serializerContext{
			WriterContext: &WriterContext{},
		},
	}

	result, err := toParameterDefinition(&context, paramName, &value.ValueParameter{
		Value: paramValue,
	})

	assert.NilError(t, err, "to parameter definiton should return no error: %s", err)
	assert.Assert(t, result != nil, "result should not be nil")

	assert.Assert(t, result == paramValue, "result should be value `%s`, but was `%v`", paramValue, result)
}

func TestToParameterDefinitionShouldWithShortSyntaxActiveShouldDoNormalWhenParameterIsMap(t *testing.T) {
	paramName := "test-param-1"
	paramValue := map[string]interface{}{
		"name": "hansi",
	}

	context := detailedSerializerContext{
		serializerContext: &serializerContext{
			WriterContext: &WriterContext{
				ParametersSerde: map[string]parameter.ParameterSerDe{
					value.ValueParameterType: value.ValueParameterSerde,
				},
			},
		},
	}

	result, err := toParameterDefinition(&context, paramName, &value.ValueParameter{
		Value: paramValue,
	})

	assert.NilError(t, err, "to parameter definiton should return no error: %s", err)
	assert.Assert(t, result != nil, "result should not be nil")

	resultMap, ok := result.(map[string]interface{})

	assert.Assert(t, ok, "result should be map")
	assert.Assert(t, resultMap["type"] == value.ValueParameterType, "result map should be of type `%s`, but was `%s`",
		value.ValueParameterType, resultMap["type"])
}

func TestForSamePropertiesWithNothingSet(t *testing.T) {
	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{
				Name:     nil,
				Template: "",
				Skip:     nil,
			},
		},
		{
			configDefinition: configDefinition{
				Name:     nil,
				Template: "",
				Skip:     nil,
			},
		},
		{
			configDefinition: configDefinition{
				Name:     nil,
				Template: "",
				Skip:     nil,
			},
		},
	}

	result := testForSameProperties(configs)

	assertPropertyCheckResult(t, propertyCheckResult{
		shareName: true,
		foundName: false,
		name:      nil,

		shareTemplate: true,
		foundTemplate: false,
		template:      "",

		shareSkip: true,
		foundSkip: false,
		skip:      nil,
	}, result)
}

func TestForSamePropertiesWithAllShared(t *testing.T) {
	name := "name"
	template := "test.json"
	skip := false

	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{
				Name:     name,
				Template: template,
				Skip:     skip,
			},
		},
		{
			configDefinition: configDefinition{
				Name:     name,
				Template: template,
				Skip:     skip,
			},
		},
		{
			configDefinition: configDefinition{
				Name:     name,
				Template: template,
				Skip:     skip,
			},
		},
	}

	result := testForSameProperties(configs)

	assertPropertyCheckResult(t, propertyCheckResult{
		shareName: true,
		foundName: true,
		name:      name,

		shareTemplate: true,
		foundTemplate: true,
		template:      template,

		shareSkip: true,
		foundSkip: true,
		skip:      skip,
	}, result)
}

func TestForSamePropertiesWithNameNotSharedByAll(t *testing.T) {
	sharedName := "name"

	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{
				Name: sharedName,
			},
		},
		{
			configDefinition: configDefinition{
				Name: nil,
			},
		},
		{
			configDefinition: configDefinition{
				Name: sharedName,
			},
		},
	}

	result := testForSameProperties(configs)

	assertPropertyCheckResult(t, propertyCheckResult{
		shareName: false,
		foundName: true,

		shareTemplate: true,
		shareSkip:     true,
	}, result)
}

func TestForSamePropertiesWithSkipNotSetExceptForOne(t *testing.T) {
	configs := []extendedConfigDefinition{
		{
			configDefinition: configDefinition{},
		},
		{
			configDefinition: configDefinition{
				Skip: true,
			},
		},
		{
			configDefinition: configDefinition{},
		},
	}

	result := testForSameProperties(configs)

	assertPropertyCheckResult(t, propertyCheckResult{
		shareName:     true,
		shareTemplate: true,

		shareSkip: false,
		foundSkip: true,
	}, result)
}

func assertPropertyCheckResult(t *testing.T, expected propertyCheckResult, actual propertyCheckResult) {
	assert.DeepEqual(t, expected.foundName, actual.foundName)
	assert.DeepEqual(t, expected.foundTemplate, actual.foundTemplate)
	assert.DeepEqual(t, expected.foundSkip, actual.foundSkip)

	assert.DeepEqual(t, expected.shareName, actual.shareName)
	assert.DeepEqual(t, expected.shareTemplate, actual.shareTemplate)
	assert.DeepEqual(t, expected.shareSkip, actual.shareSkip)

	assert.DeepEqual(t, expected.name, actual.name)
	assert.DeepEqual(t, expected.template, actual.template)
	assert.DeepEqual(t, expected.skip, actual.skip)
}

func TestWriteConfigs(t *testing.T) {

	log.Default().SetLevel(log.LevelDebug)

	var tests = []struct {
		name                  string
		configs               []Config
		expectedConfigs       map[string]topLevelDefinition
		expectedTemplatePaths []string
	}{
		{
			name: "Simple classic API write",
			configs: []Config{
				{
					Template: template.CreateTemplateFromString("project/alerting-profile/a.json", ""),
					Coordinate: coordinate.Coordinate{
						Project:  "project",
						Type:     "alerting-profile",
						ConfigId: "configId",
					},
					Type: ClassicApiType{
						Api: "alerting-profile",
					},
					Parameters: map[string]parameter.Parameter{
						NameParameter: &value.ValueParameter{Value: "name"},
					},
					SkipForConversion: envParam.New("ENV_VAR_SKIP"),
				},
			},
			expectedConfigs: map[string]topLevelDefinition{
				"alerting-profile": {
					Configs: []topLevelConfigDefinition{
						{
							Id: "configId",
							Config: configDefinition{
								Name:       "name",
								Parameters: nil,
								Template:   "a.json",
								Skip: map[any]any{
									"type": "environment",
									"name": "ENV_VAR_SKIP",
								},
							},
							Type: typeDefinition{
								Api: "alerting-profile",
							},
						},
					},
				},
			},
			expectedTemplatePaths: []string{
				"project/alerting-profile/a.json",
				"project/alerting-profile/config.yaml",
			},
		},
		{
			name: "Settings 2.0 schema write sanitizes names",
			configs: []Config{
				{
					Template: template.NewDownloadTemplate("a", "", ""),
					Coordinate: coordinate.Coordinate{
						Project:  "project",
						Type:     "builtin:alerting-profile",
						ConfigId: "configId",
					},
					Type: SettingsType{
						SchemaId: "builtin:alerting-profile",
					},
					Parameters: map[string]parameter.Parameter{
						NameParameter:  &value.ValueParameter{Value: "name"},
						ScopeParameter: value.New("tenant"),
					},
					SkipForConversion: value.New("true"),
				},
			},
			expectedConfigs: map[string]topLevelDefinition{
				"builtinalerting-profile": {
					Configs: []topLevelConfigDefinition{
						{
							Id: "configId",
							Config: configDefinition{
								Name:       "name",
								Parameters: nil,
								Template:   "a.json",
								Skip:       "true",
							},
							Type: typeDefinition{
								Settings: settingsDefinition{
									Schema: "builtin:alerting-profile",
									Scope:  "tenant",
								},
							},
						},
					},
				},
			},
			expectedTemplatePaths: []string{
				"project/builtinalerting-profile/config.yaml",
				"project/builtinalerting-profile/a.json",
			},
		},
		{
			name: "Simple settings 2.0 write",
			configs: []Config{
				{
					Template: template.CreateTemplateFromString("project/schemaid/a.json", ""),
					Coordinate: coordinate.Coordinate{
						Project:  "project",
						Type:     "schemaid",
						ConfigId: "configId",
					},
					Type: SettingsType{
						SchemaId:      "schemaid",
						SchemaVersion: "1.2.3",
					},
					Parameters: map[string]parameter.Parameter{
						ScopeParameter: &value.ValueParameter{Value: "scope"},
						NameParameter:  &value.ValueParameter{Value: "name"},
					},
					Skip: true,
				},
			},
			expectedConfigs: map[string]topLevelDefinition{
				"schemaid": {
					Configs: []topLevelConfigDefinition{
						{
							Id: "configId",
							Config: configDefinition{
								Name:       "name",
								Parameters: nil,
								Template:   "a.json",
								Skip:       true,
							},
							Type: typeDefinition{
								Settings: settingsDefinition{
									Schema:        "schemaid",
									SchemaVersion: "1.2.3",
									Scope:         "scope",
								},
							},
						},
					},
				},
			},
			expectedTemplatePaths: []string{
				"project/schemaid/a.json",
			},
		},
		{
			name: "Reference scope",
			configs: []Config{
				{
					Template: template.CreateTemplateFromString("project/schemaid/a.json", ""),
					Coordinate: coordinate.Coordinate{
						Project:  "project",
						Type:     "schemaid",
						ConfigId: "configId",
					},
					Type: SettingsType{
						SchemaId:      "schemaid",
						SchemaVersion: "1.2.3",
					},
					Parameters: map[string]parameter.Parameter{
						ScopeParameter: refParam.New("otherproject", "type", "id", "prop"),
						NameParameter:  &value.ValueParameter{Value: "name"},
					},
					Skip: false,
				},
			},
			expectedConfigs: map[string]topLevelDefinition{
				"schemaid": {
					Configs: []topLevelConfigDefinition{
						{
							Id: "configId",
							Config: configDefinition{
								Name:       "name",
								Parameters: nil,
								Template:   "a.json",
								Skip:       false,
							},
							Type: typeDefinition{
								Settings: settingsDefinition{
									Schema:        "schemaid",
									SchemaVersion: "1.2.3",
									Scope: map[any]any{
										"type":       "reference",
										"configType": "type",
										"project":    "otherproject",
										"property":   "prop",
										"configId":   "id",
									},
								},
							},
						},
					},
				},
			},
			expectedTemplatePaths: []string{
				"project/schemaid/a.json",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := testutils.TempFs(t)

			errs := WriteConfigs(&WriterContext{
				Fs:              fs,
				OutputFolder:    "test",
				ProjectFolder:   "project",
				ParametersSerde: DefaultParameterParsers,
			}, tc.configs)
			errutils.PrintErrors(errs)
			assert.Equal(t, len(errs), 0, "Writing configs should not produce an error")

			// check all api-folders config file
			for apiType, definition := range tc.expectedConfigs {

				content, err := afero.ReadFile(fs, "test/project/"+apiType+"/config.yaml")
				assert.NilError(t, err, "reading config file should not produce an error")

				var s topLevelDefinition
				err = yaml.Unmarshal(content, &s)
				assert.NilError(t, err, "unmarshalling config file should not produce an error")

				assert.DeepEqual(t, s, definition)
			}

			// check that templates have been created
			for _, path := range tc.expectedTemplatePaths {
				expectedPath := filepath.Join("test", path)
				found, err := afero.Exists(fs, expectedPath)
				assert.NilError(t, err)
				assert.Equal(t, found, true, "could not find %q", expectedPath)
			}

		})
	}

}
