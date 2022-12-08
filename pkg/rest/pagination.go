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

package rest

import (
	"net/url"
	"strings"
)

//This pagination util handles 'standard' api/v2 Dynatrace pagination.
//These APIs will contain "totalCount", "pageSize" and "nextPageKey" in their response body.
//On requests for subsequent pages, nextPage MUST be the only query parameter all other have to be omitted.

func isPaginatedResponse(jsonResponse map[string]interface{}) (paginated bool, pageKey string) {
	if jsonResponse["nextPageKey"] != nil {
		return true, jsonResponse["nextPageKey"].(string)
	}
	return false, ""
}

func addNextPageQueryParams(u *url.URL, nextPage string) *url.URL {
	queryParams := u.Query()

	if isApiV2Url(u) {
		// api/v2 requires all previously sent query params to be omitted when nextPageKey is set
		queryParams = url.Values{}
	}

	queryParams.Set("nextPageKey", nextPage)
	u.RawQuery = queryParams.Encode()
	return u
}

func isApiV2Url(url *url.URL) bool {
	return strings.Contains(url.Path, "api/v2")
}
