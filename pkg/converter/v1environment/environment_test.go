//go:build unit

/*
 * @license
 * Copyright 2023 Dynatrace LLC
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1environment

import (
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/template"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gotest.tools/assert"
)

const testYamlEnvironment = `
development:
    - name: "Dev"
    - env-url: "https://url/to/dev/environment"
    - env-token-name: "DEV"
hardening:
    - name: "Hardening"
    - env-url: "https://url/to/hardening/environment"
    - env-token-name: "HARDENING"
production.prod-environment:
    - name: "prod-environment"
    - env-url: "https://url/to/production/environment"
    - env-token-name: "PRODUCTION"
`

const testYamlEnvironmentWithGroups = `
.dev:
    - name: "Dev"
    - env-url: "https://url/to/dev/environment"
    - env-token-name: "DEV"
stage.:
    - name: "Stage"
    - env-url: "https://url/to/stage/environment"
    - env-token-name: "STAGE"
test.group.hardening:
    - name: "Hardening"
    - env-url: "https://url/to/hardening/environment"
    - env-token-name: "HARDENING"
production.prod-environment:
    - name: "prod-environment"
    - env-url: "https://url/to/production/environment"
    - env-token-name: "PRODUCTION"
`

const testYamlEnvironmentSameIds = `
development.myenvironment:
  - name: "myDevEnvironment"
  - env-url: "https://myenvironment1.dynatrace.com"
  - env-token-name: "MYENV_TOKEN"
production.myenvironment:
  - name: "myProdEnvironment"
  - env-url: "https://myenvironment2.dynatrace.com"
  - env-token-name: "MYENV_TOKEN"
`

const testYamlEnvironmentWithNewPropertyFormat = `
development:
    - name: "Dev"
    - env-url: "{{.Env.URL}}"
    - env-token-name: "DEV"
`

var testDevEnvironment = NewEnvironmentV1("development", "Dev", "", "https://url/to/dev/environment", "DEV")
var testHardeningEnvironment = NewEnvironmentV1("hardening", "Hardening", "", "https://url/to/hardening/environment", "HARDENING")
var testProductionEnvironment = NewEnvironmentV1("prod-environment", "prod-environment", "production", "https://url/to/production/environment", "PRODUCTION")
var testTrailingSlashEnvironment = NewEnvironmentV1("trailing-slash-environment", "trailing-slash-environment", "", "https://url/to/production/environment/", "TRAILINGSLASH")

func TestShouldParseYaml(t *testing.T) {

	result, e := template.UnmarshalYaml(testYamlEnvironment, "test-yaml")
	assert.NilError(t, e)

	environments, errorList := newEnvironmentsV1(result)

	assert.Check(t, len(errorList) == 0)
	assert.Check(t, len(environments) == 3)

	dev := environments["development"]
	hardening := environments["hardening"]
	production := environments["prod-environment"]

	assert.Check(t, dev != nil)
	assert.Check(t, hardening != nil)
	assert.Check(t, production != nil)

	assert.DeepEqual(t, dev, testDevEnvironment, cmp.AllowUnexported(EnvironmentV1{}))
	assert.DeepEqual(t, hardening, testHardeningEnvironment, cmp.AllowUnexported(EnvironmentV1{}))
	assert.DeepEqual(t, production, testProductionEnvironment, cmp.AllowUnexported(EnvironmentV1{}))
}

func TestParsingEnvironmentsWithMultipleGroups(t *testing.T) {
	result, e := template.UnmarshalYaml(testYamlEnvironmentWithGroups, "test-yaml")
	assert.NilError(t, e)

	environments, errorList := newEnvironmentsV1(result)
	assert.Check(t, len(errorList) == 3)
	assert.Check(t, len(environments) == 1)

	production := environments["prod-environment"]
	assert.Check(t, production != nil)
	assert.DeepEqual(t, production, testProductionEnvironment, cmp.AllowUnexported(EnvironmentV1{}))

}

func TestParsingEnvironmentsWithSameIds(t *testing.T) {
	result, e := template.UnmarshalYaml(testYamlEnvironmentSameIds, "test-yaml")
	assert.NilError(t, e)

	environments, errorList := newEnvironmentsV1(result)
	assert.Check(t, len(errorList) == 1)
	assert.Check(t, len(environments) == 1)

	myenvironment := environments["myenvironment"]
	assert.Check(t, myenvironment != nil)
}

func TestUrlAvailableWithTemplating(t *testing.T) {

	t.Setenv("URL", "1234")
	e, devEnvironment := setupEnvironment(t, testYamlEnvironmentWithNewPropertyFormat, "development")

	assert.NilError(t, e)
	assert.Equal(t, "1234", devEnvironment.GetEnvironmentUrl())

}

func TestTokenNotAvailableOnGetterCallWithTemplating(t *testing.T) {

	_, e := template.UnmarshalYaml(testYamlEnvironmentWithNewPropertyFormat, "test-yaml")
	assert.ErrorContains(t, e, "map has no entry for key \"URL\"")
}

func TestTrailingSlashTrimmedFromEnvironmentURL(t *testing.T) {
	envURL := testTrailingSlashEnvironment.GetEnvironmentUrl()
	last_char := envURL[len(envURL)-1:]

	if last_char == "/" {
		t.Errorf("Env URL is: %s; Last Char is: %s. Expected last character NOT to be a trailing slash.", envURL, last_char)
	}
}

func setupEnvironment(t *testing.T, environmentYamlContent string, environmentOfInterest string) (error, *EnvironmentV1) {

	result, e := template.UnmarshalYaml(environmentYamlContent, "test-yaml")
	assert.NilError(t, e)

	environments, errorList := newEnvironmentsV1(result)
	assert.Check(t, len(errorList) == 0)

	devEnvironment := environments[environmentOfInterest]
	assert.Check(t, devEnvironment != nil)

	return e, devEnvironment
}
