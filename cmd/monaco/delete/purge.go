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

package delete

import (
	"errors"
	"fmt"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/api"
	configDelete "github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/delete/v2"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/manifest"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/util"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/util/log"
	"github.com/spf13/afero"
	"path/filepath"
)

func Purge(fs afero.Fs, deploymentManifestPath string, environmentNames []string) error {

	deploymentManifestPath = filepath.Clean(deploymentManifestPath)
	deploymentManifestPath, manifestErr := filepath.Abs(deploymentManifestPath)

	if manifestErr != nil {
		return fmt.Errorf("error while finding absolute path for `%s`: %s", deploymentManifestPath, manifestErr)
	}

	apis := api.NewApis()

	manifest, manifestLoadError := manifest.LoadManifest(&manifest.ManifestLoaderContext{
		Fs:           fs,
		ManifestPath: deploymentManifestPath,
	})

	if manifestLoadError != nil {
		util.PrintErrors(manifestLoadError)
		return errors.New("error while loading manifest")
	}

	environments, err := manifest.FilterEnvironmentsByNames(environmentNames)
	if err != nil {
		return fmt.Errorf("Failed to load environments: %s", err)
	}

	deleteErrors := purgeConfigs(environments, apis)

	for _, e := range deleteErrors {
		log.Error("Deletion error: %s", e)
	}
	if len(deleteErrors) > 0 {
		return fmt.Errorf("encountered %v errors during delete", len(deleteErrors))
	}
	return nil
}

func purgeConfigs(environments []manifest.EnvironmentDefinition, apis map[string]api.Api) (errors []error) {

	for _, env := range environments {
		deleteErrors := purgeConfigsForEnvironment(env, apis)

		if deleteErrors != nil {
			errors = append(errors, deleteErrors...)
		}
	}

	return errors
}

func purgeConfigsForEnvironment(env manifest.EnvironmentDefinition, apis map[string]api.Api) []error {
	dynatraceClient, err := createClient(env, false)

	if err != nil {
		return []error{
			fmt.Errorf("failed to create a client for env `%s` due to the following error: %s", env.Name, err),
		}
	}

	log.Info("Deleting configs for environment `%s`", env.Name)

	return configDelete.DeleteAllConfigs(dynatraceClient, apis)
}