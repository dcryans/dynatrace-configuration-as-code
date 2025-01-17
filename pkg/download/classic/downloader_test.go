//go:build unit

// @license
// Copyright 2022 Dynatrace LLC
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

package classic

import (
	"fmt"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/api"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/client"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDownloadAllConfigs_FailedToFindConfigsToDownload(t *testing.T) {
	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).Return([]client.Value{}, fmt.Errorf("NO"))
	downloader := NewDownloader(c)
	testAPI := api.API{ID: "API_ID", URLPath: "API_PATH", NonUniqueName: true}
	apiMap := api.APIs{"API_ID": testAPI}

	assert.Len(t, downloader.DownloadAll(apiMap, "project"), 0)
}

func TestDownloadAll_NoConfigsToDownloadFound(t *testing.T) {
	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).Return([]client.Value{}, nil)
	downloader := NewDownloader(c)
	testAPI := api.API{ID: "API_ID", URLPath: "API_PATH", NonUniqueName: true}

	apiMap := api.APIs{"API_ID": testAPI}

	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 0)
}

func TestDownloadAll_ConfigsDownloaded(t *testing.T) {
	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).DoAndReturn(func(a api.API) ([]client.Value, error) {
		if a.ID == "API_ID_1" {
			return []client.Value{{Id: "API_ID_1", Name: "API_NAME_1"}}, nil
		} else if a.ID == "API_ID_2" {
			return []client.Value{{Id: "API_ID_2", Name: "API_NAME_2"}}, nil
		}
		return nil, nil
	}).Times(2)
	downloader := NewDownloader(c)
	testAPI1 := api.API{ID: "API_ID_1", URLPath: "API_PATH_1", NonUniqueName: true}
	testAPI2 := api.API{ID: "API_ID_2", URLPath: "API_PATH_2", NonUniqueName: true}
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil)
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil)

	apiMap := api.APIs{"API_ID_1": testAPI1, "API_ID_2": testAPI2}

	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 2)
}

func TestDownloadAll_ConfigsDownloaded_WithEmptyFilter(t *testing.T) {
	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).DoAndReturn(func(a api.API) ([]client.Value, error) {
		if a.ID == "API_ID_1" {
			return []client.Value{{Id: "API_ID_1", Name: "API_NAME_1"}}, nil
		} else if a.ID == "API_ID_2" {
			return []client.Value{{Id: "API_ID_2", Name: "API_NAME_2"}}, nil
		}
		return nil, nil
	}).Times(2)
	downloader := NewDownloader(c, WithAPIFilters(map[string]apiFilter{}))
	testAPI1 := api.API{ID: "API_ID_1", URLPath: "API_PATH_1", NonUniqueName: true}
	testAPI2 := api.API{ID: "API_ID_2", URLPath: "API_PATH_2", NonUniqueName: true}
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil)
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil)

	apiMap := api.APIs{"API_ID_1": testAPI1, "API_ID_2": testAPI2}

	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 2)
}

func TestDownloadAll_SingleConfigurationAPI(t *testing.T) {
	client := client.NewMockClient(gomock.NewController(t))
	client.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil)
	downloader := NewDownloader(client)
	testAPI1 := api.API{ID: "API_ID_1", URLPath: "API_PATH_1", SingleConfiguration: true, NonUniqueName: true}
	apiMap := api.APIs{"API_ID_1": testAPI1}

	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 1)
}

func TestDownloadAll_ErrorFetchingConfig(t *testing.T) {
	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).DoAndReturn(func(a api.API) ([]client.Value, error) {
		if a.ID == "API_ID_1" {
			return []client.Value{{Id: "API_ID_1", Name: "API_NAME_1"}}, nil
		} else if a.ID == "API_ID_2" {
			return []client.Value{{Id: "API_ID_2", Name: "API_NAME_2"}}, nil
		}
		return nil, nil
	}).Times(2)

	downloader := NewDownloader(c)

	testAPI1 := api.API{ID: "API_ID_1", URLPath: "API_PATH_1", NonUniqueName: true}
	testAPI2 := api.API{ID: "API_ID_2", URLPath: "API_PATH_2", NonUniqueName: true}

	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).DoAndReturn(func(a api.API, id string) (json []byte, err error) {
		if a.ID == "API_ID_1" {
			return []byte("{}"), fmt.Errorf("NO")
		}
		return []byte("{}"), nil
	}).Times(2)

	apiMap := api.APIs{"API_ID_1": testAPI1, "API_ID_2": testAPI2}
	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 1)
}

func TestDownloadAll_SkipConfigThatShouldNotBePersisted(t *testing.T) {

	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).DoAndReturn(func(a api.API) ([]client.Value, error) {
		if a.ID == "API_ID_1" {
			return []client.Value{{Id: "API_ID_1", Name: "API_NAME_1"}}, nil
		} else if a.ID == "API_ID_2" {
			return []client.Value{{Id: "API_ID_2", Name: "API_NAME_2"}}, nil
		}
		return nil, nil
	}).Times(2)

	apiFilters := map[string]apiFilter{"API_ID_1": {
		shouldConfigBePersisted: func(_ map[string]interface{}) bool {
			return false
		},
	}}
	downloader := NewDownloader(c, WithAPIFilters(apiFilters))

	testAPI1 := api.API{ID: "API_ID_1", URLPath: "API_PATH_1", NonUniqueName: true}
	testAPI2 := api.API{ID: "API_ID_2", URLPath: "API_PATH_2", NonUniqueName: true}
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil).Times(2)

	apiMap := api.APIs{"API_ID_1": testAPI1, "API_ID_2": testAPI2}

	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 1)
}

func TestDownloadAll_SkipConfigBeforeDownload(t *testing.T) {

	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).DoAndReturn(func(a api.API) ([]client.Value, error) {
		if a.ID == "API_ID_1" {
			return []client.Value{{Id: "API_ID_1", Name: "API_NAME_1"}}, nil
		} else if a.ID == "API_ID_2" {
			return []client.Value{{Id: "API_ID_2", Name: "API_NAME_2"}}, nil
		}
		return nil, nil
	}).Times(2)

	apiFilters := map[string]apiFilter{"API_ID_1": {
		shouldBeSkippedPreDownload: func(_ client.Value) bool {
			return true
		},
	}}
	downloader := NewDownloader(c, WithAPIFilters(apiFilters))

	testAPI1 := api.API{ID: "API_ID_1", URLPath: "API_PATH_1", NonUniqueName: true}
	testAPI2 := api.API{ID: "API_ID_2", URLPath: "API_PATH_2", NonUniqueName: true}
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil)

	apiMap := api.APIs{"API_ID_1": testAPI1, "API_ID_2": testAPI2}

	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 1)
}

func TestDownloadAll_EmptyAPIMap_NothingIsDownloaded(t *testing.T) {
	client := client.NewMockClient(gomock.NewController(t))
	downloader := NewDownloader(client)

	configurations := downloader.DownloadAll(api.APIs{}, "project")
	assert.Len(t, configurations, 0)
}

func TestDownloadAll_APIWithoutAnyConfigAvailableAreNotDownloaded(t *testing.T) {
	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).DoAndReturn(func(a api.API) ([]client.Value, error) {
		if a.ID == "API_ID_1" {
			return []client.Value{{Id: "API_ID_1", Name: "API_NAME_1"}}, nil
		} else if a.ID == "API_ID_2" {
			return []client.Value{}, nil
		}
		return nil, nil
	}).Times(2)
	downloader := NewDownloader(c)
	testAPI1 := api.API{ID: "API_ID_1", URLPath: "API_PATH_1", NonUniqueName: true}
	testAPI2 := api.API{ID: "API_ID_2", URLPath: "API_PATH_2", NonUniqueName: true}
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil)

	apiMap := api.APIs{"API_ID_1": testAPI1, "API_ID_2": testAPI2}

	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 1)
}

func TestDownloadAll_MalformedResponseFromAnAPI(t *testing.T) {
	c := client.NewMockClient(gomock.NewController(t))
	c.EXPECT().ListConfigs(gomock.Any()).DoAndReturn(func(a api.API) ([]client.Value, error) {
		if a.ID == "API_ID_1" {
			return []client.Value{{Id: "API_ID_1", Name: "API_NAME_1"}}, nil
		} else if a.ID == "API_ID_2" {
			return []client.Value{{Id: "API_ID_2", Name: "API_NAME_2"}}, nil
		}
		return nil, nil
	}).Times(2)
	downloader := NewDownloader(c)
	testAPI1 := api.API{ID: "API_ID_1", URLPath: "API_PATH_1", NonUniqueName: true}
	testAPI2 := api.API{ID: "API_ID_2", URLPath: "API_PATH_2", NonUniqueName: true}
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("-1"), nil)
	c.EXPECT().ReadConfigById(gomock.Any(), gomock.Any()).Return([]byte("{}"), nil)

	apiMap := api.APIs{"API_ID_1": testAPI1, "API_ID_2": testAPI2}

	configurations := downloader.DownloadAll(apiMap, "project")
	assert.Len(t, configurations, 1)
}
