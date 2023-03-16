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

package match

import (
	"fmt"
	"time"

	config "github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match"
	matchEntities "github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/entities"
	project "github.com/dynatrace/dynatrace-configuration-as-code/pkg/project/v2"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/util"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/util/log"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/util/maps"
	"github.com/spf13/afero"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func Match(fs afero.Fs, matchFileName string) error {

	startTime := time.Now()

	matchParameters, err := match.LoadMatchingParameters(fs, matchFileName)
	if err != nil {
		return err
	}

	configsSource, configsTarget, err := loadProjects(fs, matchParameters)
	if err != nil {
		return err
	}

	stats, nbEntitiesSource, nbEntitiesTarget, err := matchEntities.CompareEntities(fs, matchParameters, configsSource, configsTarget)
	if err != nil {
		return err
	}

	for _, stat := range stats {
		log.Info(stat)
	}

	p := message.NewPrinter(language.English)
	log.Info("Finished matching %d entity types, %s source entities and %s target entities in %v",
		len(configsSource), p.Sprintf("%d", nbEntitiesSource), p.Sprintf("%d", nbEntitiesTarget), time.Since(startTime))

	return nil
}

func loadProjects(fs afero.Fs, matchParameters match.MatchParameters) (project.ConfigsPerType, project.ConfigsPerType, error) {

	sourceConfigs, err := loadProject(fs, matchParameters.Source)
	if err != nil {
		return nil, nil, err
	}

	targetConfigs, err := loadProject(fs, matchParameters.Target)
	if err != nil {
		return nil, nil, err
	}

	return sourceConfigs, targetConfigs, nil

}

func loadProject(fs afero.Fs, env match.MatchParametersEnv) (project.ConfigsPerType, error) {

	log.Info("Loading project %s of %s environment %s ...", env.Project, env.EnvType, env.Environment)

	context := project.ProjectLoaderContext{
		KnownApis:       nil,
		WorkingDir:      env.WorkingDir,
		Manifest:        env.Manifest,
		ParametersSerde: config.DefaultParameterParsers,
	}

	projects, errs := project.LoadProjectsSpecific(fs, context, []string{env.Project}, []string{env.Environment})

	if errs != nil {
		return nil, util.PrintAndFormatErrors(errs, "could not load projects from manifest")
	}

	nbProjects := len(projects)
	if nbProjects != 1 {
		return nil, fmt.Errorf("loaded %d projects for project: %s and environment: %s, expected 1 project to compare for %s environment",
			nbProjects, env.Project, env.Environment, env.EnvType)
	}

	project := projects[0]
	nbEnvsInProject := len(maps.Keys(project.Configs))
	if nbEnvsInProject != 1 {
		return nil, fmt.Errorf("loaded %d environments for project: %s and environment: %s, expected 1 environment to compare for %s environment: List: %v",
			nbEnvsInProject, env.Project, env.Environment, env.EnvType, maps.Keys(project.Configs))
	}

	envConfigs := project.Configs[env.Environment]

	log.Info("Loaded %d entity types for %s environment %s, entity types: %v",
		len(maps.Keys(envConfigs)), env.EnvType, env.Environment, maps.Keys(envConfigs))

	return envConfigs, nil

}