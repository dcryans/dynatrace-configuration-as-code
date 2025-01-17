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
	"github.com/dynatrace/dynatrace-configuration-as-code/cmd/monaco/cmdutils"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/errutils"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/log"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/maps"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/delete"
	"path/filepath"

	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/api"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/manifest"
	"github.com/spf13/afero"
)

func Delete(fs afero.Fs, deploymentManifestPath string, deleteFile string, environmentNames []string, environmentGroups []string) error {

	deploymentManifestPath = filepath.Clean(deploymentManifestPath)
	deploymentManifestPath, manifestErr := filepath.Abs(deploymentManifestPath)

	if manifestErr != nil {
		return fmt.Errorf("error while finding absolute path for `%s`: %w", deploymentManifestPath, manifestErr)
	}

	apis := api.NewAPIs()

	manifest, manifestLoadError := manifest.LoadManifest(&manifest.LoaderContext{
		Fs:           fs,
		ManifestPath: deploymentManifestPath,
		Environments: environmentNames,
		Groups:       environmentGroups,
	})

	if manifestLoadError != nil {
		errutils.PrintErrors(manifestLoadError)
		return errors.New("error while loading manifest")
	}

	entriesToDelete, errs := delete.LoadEntriesToDelete(fs, apis.GetNames(), deleteFile)
	if errs != nil {
		return fmt.Errorf("encountered errors while parsing delete.yaml: %s", errs)
	}

	deleteErrors := deleteConfigs(maps.Values(manifest.Environments), apis, entriesToDelete)

	for _, e := range deleteErrors {
		log.Error("Deletion error: %s", e)
	}
	if len(deleteErrors) > 0 {
		return fmt.Errorf("encountered %v errors during delete", len(deleteErrors))
	}
	return nil
}

func deleteConfigs(environments []manifest.EnvironmentDefinition, apis api.APIs, entriesToDelete map[string][]delete.DeletePointer) (errors []error) {

	for _, env := range environments {
		deleteErrors := deleteConfigForEnvironment(env, apis, entriesToDelete)

		if deleteErrors != nil {
			errors = append(errors, deleteErrors...)
		}
	}

	return errors
}

func deleteConfigForEnvironment(env manifest.EnvironmentDefinition, apis api.APIs, entriesToDelete map[string][]delete.DeletePointer) []error {
	dynatraceClient, err := cmdutils.CreateDTClient(env.URL.Value, env.Auth, false)

	if err != nil {
		return []error{
			fmt.Errorf("It was not possible to create a client for env `%s` due to the following error: %w", env.Name, err),
		}
	}

	log.Info("Deleting configs for environment `%s`", env.Name)

	return delete.DeleteConfigs(dynatraceClient, apis, entriesToDelete)
}
