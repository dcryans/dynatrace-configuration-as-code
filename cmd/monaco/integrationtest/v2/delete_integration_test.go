//go:build integration

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

package v2

import (
	"fmt"
	"github.com/dynatrace/dynatrace-configuration-as-code/cmd/monaco/integrationtest"
	"github.com/dynatrace/dynatrace-configuration-as-code/cmd/monaco/runner"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/testutils"
	"github.com/spf13/afero"
	"gotest.tools/assert"
	"path/filepath"
	"testing"
)

func TestDelete(t *testing.T) {

	tests := []struct {
		name       string
		manifest   string
		deleteFile string
		cmdFlags   []string
	}{
		{
			"Default values",
			"manifest.yaml",
			"delete.yaml",
			[]string{},
		},
		{
			"Specific manifest",
			"my_special_manifest.yaml",
			"delete.yaml",
			[]string{"--manifest", "my_special_manifest.yaml"},
		},
		{
			"Specific manifest (shorthand)",
			"my_special_manifest.yaml",
			"delete.yaml",
			[]string{"-m", "my_special_manifest.yaml"},
		},
		{
			"Specific delete file",
			"manifest.yaml",
			"super-special-removal-file.yaml",
			[]string{"--file", "super-special-removal-file.yaml"},
		},
		{
			"Specific manifest and delete file",
			"my_special_manifest.yaml",
			"super-special-removal-file.yaml",
			[]string{"--manifest", "my_special_manifest.yaml", "--file", "super-special-removal-file.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t1 *testing.T) {
			configFolder := "test-resources/delete-test-configs/"
			deployManifestPath := configFolder + "deploy-manifest.yaml"

			fs := testutils.CreateTestFileSystem()

			//create config yaml
			cfgTemplate := "configs:\n- id: %s\n  type:\n    settings:\n      schema: builtin:tags.auto-tagging\n      scope: environment\n  config:\n    name: %s\n    template: auto-tag-setting.json\n"
			cfgId := fmt.Sprintf("deleteSample_%s", integrationtest.GenerateTestSuffix(tt.name))
			configContent := fmt.Sprintf(cfgTemplate, cfgId, cfgId)

			configYamlPath, err := filepath.Abs(filepath.Join(configFolder, "project", "config.yaml"))
			assert.NilError(t, err)
			err = afero.WriteFile(fs, configYamlPath, []byte(configContent), 644)
			assert.NilError(t, err)

			//create delete yaml
			deleteContent := fmt.Sprintf("delete:\n  - \"builtin:tags.auto-tagging/%s\"", cfgId)
			deleteYamlPath, err := filepath.Abs(tt.deleteFile)
			assert.NilError(t, err)
			err = afero.WriteFile(fs, deleteYamlPath, []byte(deleteContent), 644)
			assert.NilError(t, err)

			//create manifest file
			manifestContent, err := afero.ReadFile(fs, deployManifestPath)
			assert.NilError(t, err)
			manifestPath, err := filepath.Abs(tt.manifest)
			err = afero.WriteFile(fs, manifestPath, manifestContent, 644)
			assert.NilError(t, err)

			// DEPLOY Config
			cmd := runner.BuildCli(fs)
			cmd.SetArgs([]string{"deploy", "--verbose", deployManifestPath})
			err = cmd.Execute()
			assert.NilError(t, err)
			integrationtest.AssertAllConfigsAvailability(t, fs, deployManifestPath, []string{}, "", true)

			// DELETE Config
			cmd = runner.BuildCli(fs)
			baseCmd := []string{"delete", "--verbose"}
			cmd.SetArgs(append(baseCmd, tt.cmdFlags...))
			err = cmd.Execute()
			assert.NilError(t, err)
			integrationtest.AssertAllConfigsAvailability(t, fs, deployManifestPath, []string{}, "", false)

		})
	}
}
