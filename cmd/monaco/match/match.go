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

	"github.com/dynatrace/dynatrace-configuration-as-code/internal/errutils"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/log"
	"github.com/dynatrace/dynatrace-configuration-as-code/internal/maps"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/api"
	config "github.com/dynatrace/dynatrace-configuration-as-code/pkg/config/v2"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/match"
	matchConfigs "github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/configs"
	matchEntities "github.com/dynatrace/dynatrace-configuration-as-code/pkg/match/entities"
	project "github.com/dynatrace/dynatrace-configuration-as-code/pkg/project/v2"
	"github.com/spf13/afero"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

//go:generate mockgen -source=match.go -destination=match_mock.go -package=match -write_package_comment=false Command

// Command is used to test the CLi commands properly without executing the actual monaco match.
//
// The actual implementations are in the [DefaultCommand] struct.
type Command interface {
	Match(fs afero.Fs, matchFileName string) error
}

// DefaultCommand is used to implement the [Command] interface.
type DefaultCommand struct{}

// make sure DefaultCommand implements the Command interface
var (
	_ Command = (*DefaultCommand)(nil)
)

func (d DefaultCommand) Match(fs afero.Fs, matchFileName string) error {

	startTime := time.Now()

	matchParameters, err := match.LoadMatchingParameters(fs, matchFileName)
	if err != nil {
		return err
	}

	configsSource, configsTarget, err := loadProjects(fs, matchParameters)
	if err != nil {
		return err
	}

	if matchParameters.Type == "entities" {

		err = runAndPrintMatchEntities(fs, matchParameters, configsSource, configsTarget, startTime)
		if err != nil {
			return err
		}

	} else if matchParameters.Type == "configs" {

		err = runAndPrintMatchConfigs(fs, matchParameters, configsSource, configsTarget, startTime)
		if err != nil {
			return err
		}

	}

	return nil
}

func runAndPrintMatchEntities(fs afero.Fs, matchParameters match.MatchParameters, configsSource project.ConfigsPerType, configsTarget project.ConfigsPerType, startTime time.Time) error {

	stats, entitiesSourceCount, entitiesTargetCount, err := matchEntities.MatchEntities(fs, matchParameters, configsSource, configsTarget)
	if err != nil {
		return err
	}

	for _, stat := range stats {
		log.Info(stat)
	}

	p := message.NewPrinter(language.English)
	log.Info("Finished matching %d entity types, %s source entities and %s target entities in %v",
		len(configsSource), p.Sprintf("%d", entitiesSourceCount), p.Sprintf("%d", entitiesTargetCount), time.Since(startTime))

	return nil
}

func runAndPrintMatchConfigs(fs afero.Fs, matchParameters match.MatchParameters, configsSource project.ConfigsPerType, configsTarget project.ConfigsPerType, startTime time.Time) error {

	stats, configsSourceCount, configsTargetCount, err := matchConfigs.MatchConfigs(fs, matchParameters, configsSource, configsTarget)
	if err != nil {
		return err
	}

	for _, stat := range stats {
		log.Info(stat)
	}

	p := message.NewPrinter(language.English)
	log.Info("Finished matching %d config schemas, %s source configs and %s target configs in %v",
		len(configsSource), p.Sprintf("%d", configsSourceCount), p.Sprintf("%d", configsTargetCount), time.Since(startTime))

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
		KnownApis:       api.NewAPIs().GetApiNameLookup(),
		WorkingDir:      env.WorkingDir,
		Manifest:        env.Manifest,
		ParametersSerde: config.DefaultParameterParsers,
	}

	projects, errs := project.LoadProjectsSpecific(fs, context, []string{env.Project}, []string{env.Environment})

	if errs != nil {
		return nil, errutils.PrintAndFormatErrors(errs, "could not load projects from manifest")
	}

	projectCount := len(projects)
	if projectCount != 1 {
		return nil, fmt.Errorf("loaded %d projects for project: %s and environment: %s, expected 1 project to compare for %s environment",
			projectCount, env.Project, env.Environment, env.EnvType)
	}

	project := projects[0]
	envsInProjectCount := len(maps.Keys(project.Configs))
	if envsInProjectCount != 1 {
		return nil, fmt.Errorf("loaded %d environments for project: %s and environment: %s, expected 1 environment to compare for %s environment: List: %v",
			envsInProjectCount, env.Project, env.Environment, env.EnvType, maps.Keys(project.Configs))
	}

	envConfigs := project.Configs[env.Environment]

	log.Info("Loaded %d config types for %s environment %s, config types: %v",
		len(maps.Keys(envConfigs)), env.EnvType, env.Environment, maps.Keys(envConfigs))

	return envConfigs, nil

}