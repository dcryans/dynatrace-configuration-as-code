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
	"errors"
	"fmt"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/util/slices"
	"path/filepath"
	"strings"

	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/api"
	config "github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/config/v2"
	configError "github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/config/v2/errors"
	deploy "github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/deploy/v2"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/manifest"
	project "github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/project/v2"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/project/v2/topologysort"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/rest"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/util"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/util/client"
	"github.com/dynatrace-oss/dynatrace-monitoring-as-code/pkg/util/log"
	"github.com/spf13/afero"
)

func Deploy(fs afero.Fs, deploymentManifestPath string, specificEnvironments []string,
	specificProject []string, dryRun, continueOnError bool) error {

	deploymentManifestPath = filepath.Clean(deploymentManifestPath)
	deploymentManifestPath, err := filepath.Abs(deploymentManifestPath)

	if err != nil {
		return fmt.Errorf("error while finding absolute path for `%s`: %s", deploymentManifestPath, err)
	}

	manifest, errs := manifest.LoadManifest(&manifest.ManifestLoaderContext{
		Fs:           fs,
		ManifestPath: deploymentManifestPath,
	})

	if errs != nil {
		// TODO add grouping and print proper error repot
		util.PrintErrors(errs)
		return errors.New("error while loading manifest")
	}

	environments, err := manifest.FilterEnvironmentsByNames(specificEnvironments)
	if err != nil {
		return err
	}

	environmentMap := toEnvironmentMap(environments)
	environmentNames := toEnvironmentNames(environments)
	workingDir := filepath.Dir(deploymentManifestPath)

	apis := api.NewApis()

	log.Debug("Loading configuration projects ...")
	projects, errs := project.LoadProjects(fs, project.ProjectLoaderContext{
		KnownApis:       api.GetApiNameLookup(apis),
		WorkingDir:      workingDir,
		Manifest:        manifest,
		ParametersSerde: config.DefaultParameterParsers,
	})

	if errs != nil {
		printErrorReport(errs)

		return errors.New("error while loading projects - you may be loading v1 projects, please 'convert' to v2")
	}

	projects, err = loadProjectsToDeploy(specificProject, projects, environmentNames)
	if err != nil {
		return err
	}

	sortedConfigs, errs := topologysort.GetSortedConfigsForEnvironments(projects, environmentNames)

	if errs != nil {
		// TODO add grouping and print proper error repot
		util.PrintErrors(errs)
		return errors.New("error during sort")
	}

	log.Info("Projects to be deployed:")
	for _, p := range projects {
		log.Info("  - %s", p)
	}

	log.Info("Environments to deploy to:")
	for _, name := range environmentNames {
		log.Info("  - %s", name)
	}

	err = execDeployment(sortedConfigs, environmentMap, continueOnError, dryRun, apis)

	if err != nil {
		return err
	}

	return nil
}

func loadProjectsToDeploy(specificProject []string, projects []project.Project, environmentNames []string) ([]project.Project, error) {
	if len(specificProject) > 0 {
		filtered, err := filterProjectsByName(projects, specificProject)

		if err != nil {
			return nil, err
		}

		projectsWithDependencies, err := loadProjectsWithDependencies(projects, filtered, environmentNames)

		if err != nil {
			return nil, err
		}

		projects = projectsWithDependencies
	}

	return projects, nil
}

func execDeployment(sortedConfigs map[string][]config.Config, environmentMap map[string]manifest.EnvironmentDefinition, continueOnError bool, dryRun bool, apis map[string]api.Api) error {
	var deploymentErrors []error

	for envName, configs := range sortedConfigs {
		logDeploymentInfo(dryRun, envName)
		env, found := environmentMap[envName]

		if !found {
			if continueOnError {
				deploymentErrors = append(deploymentErrors, fmt.Errorf("cannot find environment `%s`", envName))
				continue
			} else {
				return fmt.Errorf("cannot find environment `%s`", envName)
			}
		}

		client, err := getClient(env, dryRun)

		if err != nil {
			if continueOnError {
				deploymentErrors = append(deploymentErrors, err)
				continue
			} else {
				return err
			}
		}

		errors := deploy.DeployConfigs(client, apis, configs, continueOnError, dryRun)

		deploymentErrors = append(deploymentErrors, errors...)
	}

	if deploymentErrors != nil {
		printErrorReport(deploymentErrors)

		return fmt.Errorf("errors during %s", getOperationNounForLogging(dryRun))
	} else {
		log.Info("%s finished without errors", getOperationNounForLogging(dryRun))
	}

	return nil
}

func logDeploymentInfo(dryRun bool, envName string) {
	if dryRun {
		log.Info("Validating configurations for environment `%s`...", envName)
	} else {
		log.Info("Deploying configurations to environment `%s`...", envName)
	}
}

func getOperationNounForLogging(dryRun bool) string {
	if dryRun {
		return "Validation"
	}
	return "Deployment"
}

func printErrorReport(deploymentErrors []error) {
	var configErrors []configError.ConfigError
	var generalErrors []error

	for _, err := range deploymentErrors {
		switch e := err.(type) {
		case configError.ConfigError:
			configErrors = append(configErrors, e)
		default:
			generalErrors = append(generalErrors, e)
		}
	}

	if len(generalErrors) > 0 {
		log.Error("=== General Errors ===")
		for _, err := range generalErrors {
			log.Error(util.ErrorString(err))
		}
	}

	groupedConfigErrors := groupConfigErrors(configErrors)

	for project, apiErrors := range groupedConfigErrors {
		for api, configErrors := range apiErrors {
			for config, errs := range configErrors {
				var generalConfigErrors []configError.ConfigError
				var detailedConfigErrors []configError.DetailedConfigError

				for _, err := range errs {
					switch e := err.(type) {
					case configError.DetailedConfigError:
						detailedConfigErrors = append(detailedConfigErrors, e)
					default:
						generalConfigErrors = append(generalConfigErrors, e)
					}
				}

				groupErrors := groupEnvironmentConfigErrors(detailedConfigErrors)

				for _, err := range generalConfigErrors {
					log.Error("%s:%s:%s %s", project, api, config, util.ErrorString(err))
				}

				for group, environmentErrors := range groupErrors {
					for env, errs := range environmentErrors {
						for _, err := range errs {
							log.Error("%s(%s) %s:%s:%s %T %s", env, group, project, api, config, err, util.ErrorString(err))
						}
					}
				}
			}
		}
	}
}

type ProjectErrors map[string]ApiErrors
type ApiErrors map[string]ConfigErrors
type ConfigErrors map[string][]configError.ConfigError

func groupConfigErrors(errors []configError.ConfigError) ProjectErrors {
	projectErrors := make(ProjectErrors)

	for _, err := range errors {
		coord := err.Coordinates()

		apiErrors := projectErrors[coord.Project]

		if apiErrors == nil {
			apiErrors = make(ApiErrors)
			apiErrors[coord.Type] = make(ConfigErrors)
			projectErrors[coord.Project] = apiErrors
		}

		configErrors := apiErrors[coord.Type]

		if configErrors == nil {
			configErrors = make(ConfigErrors)
			apiErrors[coord.Type] = configErrors
		}

		configErrors[coord.Config] = append(configErrors[coord.Config], err)
	}

	return projectErrors
}

type GroupErrors map[string]EnvironmentErrors
type EnvironmentErrors map[string][]configError.DetailedConfigError

func groupEnvironmentConfigErrors(errors []configError.DetailedConfigError) GroupErrors {
	groupErrors := make(GroupErrors)

	for _, err := range errors {
		locationDetails := err.LocationDetails()

		envErrors := groupErrors[locationDetails.Group]

		if envErrors == nil {
			envErrors = make(EnvironmentErrors)
			groupErrors[locationDetails.Group] = envErrors
		}

		envErrors[locationDetails.Environment] = append(envErrors[locationDetails.Environment], err)
	}

	return groupErrors
}

func toEnvironmentNames(environments []manifest.EnvironmentDefinition) []string {
	result := make([]string, 0, len(environments))

	for _, env := range environments {
		result = append(result, env.Name)
	}

	return result
}

func filterProjectsByName(projects []project.Project, names []string) ([]string, error) {
	var result []string

	foundProjects := map[string]struct{}{}

	for _, p := range projects {
		if containsName(names, p.Id) {
			foundProjects[p.Id] = struct{}{}
			result = append(result, p.Id)
		} else if containsName(names, p.GroupId) {
			foundProjects[p.GroupId] = struct{}{}
			result = append(result, p.Id)
		}
	}

	var notFoundProjects []string

	for _, name := range names {
		if _, found := foundProjects[name]; !found {
			notFoundProjects = append(notFoundProjects, name)
		}
	}

	if notFoundProjects != nil {
		return nil, fmt.Errorf("no project with names `%s` found", strings.Join(names, ", "))
	}

	return result, nil
}

func loadProjectsWithDependencies(projects []project.Project, projectIdsToLoad []string, environments []string) ([]project.Project, error) {
	lookupMap := toProjectMap(projects)
	alreadyChecked := map[string]struct{}{}
	toCheck := append(make([]string, 0, len(projectIdsToLoad)), projectIdsToLoad...)

	var result []project.Project
	var unknownProjects []string

	for len(toCheck) > 0 {
		current := toCheck[0]
		toCheck = toCheck[1:]

		if _, found := alreadyChecked[current]; found {
			continue
		}

		if project, found := lookupMap[current]; found {
			alreadyChecked[current] = struct{}{}
			result = append(result, project)

			// we need to load only the dependencies of environments we are going to deploy
			for _, env := range environments {
				toCheck = append(toCheck, project.Dependencies[env]...)
			}
		} else {
			unknownProjects = append(unknownProjects, current)
		}
	}

	if unknownProjects != nil {
		return nil, fmt.Errorf("error while gathering dependencies. no projects with name `%s` found", unknownProjects)
	}

	return result, nil
}

func toProjectMap(projects []project.Project) map[string]project.Project {
	result := make(map[string]project.Project)

	for _, p := range projects {
		result[p.Id] = p
	}

	return result
}

func containsName(names []string, name string) bool {
	return slices.Contains(names, name)
}

func toEnvironmentMap(environments []manifest.EnvironmentDefinition) map[string]manifest.EnvironmentDefinition {
	result := make(map[string]manifest.EnvironmentDefinition)

	for _, env := range environments {
		result[env.Name] = env
	}

	return result
}

func getClient(environment manifest.EnvironmentDefinition, dryRun bool) (rest.DynatraceClient, error) {
	if dryRun {
		return &client.DummyClient{
			Entries: map[api.Api][]client.DataEntry{},
		}, nil
	} else {
		token, err := environment.GetToken()

		if err != nil {
			return nil, err
		}

		url, err := environment.GetUrl()
		if err != nil {
			return nil, err
		}

		return rest.NewDynatraceClient(url, token)
	}
}
