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

package cmdutils

import (
	"encoding/json"
	"github.com/dynatrace/dynatrace-configuration-as-code/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVerifyClusterGen(t *testing.T) {
	type args struct {
		envs manifest.Environments
	}
	tests := []struct {
		name            string
		args            args
		versionApiFails bool
		handler         http.HandlerFunc
		wantErr         bool
	}{
		{
			name: "empty environment - passes",
			args: args{
				envs: manifest.Environments{},
			},
			wantErr: false,
		},
		{
			name: "single environment without fields set - fails",
			args: args{
				envs: manifest.Environments{},
			},
			wantErr: false,
		},
		{
			name: "environment type invalid - fails",
			args: args{
				envs: manifest.Environments{
					"env1": manifest.EnvironmentDefinition{
						Name: "env1",
						Type: -6,
						Url: manifest.UrlDefinition{
							Type:  manifest.ValueUrlType,
							Name:  "URL",
							Value: "",
						},
						Group: "",
						Auth:  manifest.Auth{},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := VerifyClusterGen(tt.args.envs); (err != nil) != tt.wantErr {
				t.Errorf("VerifyClusterGen() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	t.Run("Call classic Version EP - ok", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(200)
			_, _ = rw.Write([]byte(`{"version" : "1.262.0.20230303"}`))
		}))
		defer server.Close()

		err := VerifyClusterGen(manifest.Environments{
			"env": manifest.EnvironmentDefinition{
				Name: "env",
				Type: manifest.Classic,
				Url: manifest.UrlDefinition{
					Type:  manifest.ValueUrlType,
					Name:  "URL",
					Value: server.URL,
				},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("Call Platform Version EP - ok", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if strings.HasSuffix(req.URL.Path, "sso") {
				token := &oauth2.Token{
					AccessToken: "test-access-token",
					TokenType:   "Bearer",
					Expiry:      time.Now().Add(time.Hour),
				}

				rw.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(rw).Encode(token)
				return
			}

			rw.WriteHeader(200)
			_, _ = rw.Write([]byte(`{"version" : "1.262.0.20230303"}`))
		}))
		defer server.Close()

		ssoTokenURL = server.URL + "/sso"

		err := VerifyClusterGen(manifest.Environments{
			"env": manifest.EnvironmentDefinition{
				Name: "env",
				Type: manifest.Platform,
				Url: manifest.UrlDefinition{
					Type:  manifest.ValueUrlType,
					Name:  "URL",
					Value: server.URL,
				},
			},
		})
		assert.NoError(t, err)
	})

	t.Run("version EP not available ", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(404)
			_, _ = rw.Write([]byte(`{"version" : "1.262.0.20230303"}`))
		}))
		defer server.Close()

		err := VerifyClusterGen(manifest.Environments{
			"env": manifest.EnvironmentDefinition{
				Name: "env",
				Type: manifest.Classic,
				Url: manifest.UrlDefinition{
					Type:  manifest.ValueUrlType,
					Name:  "URL",
					Value: server.URL + "/WRONG_URL",
				},
			},
		})
		assert.Error(t, err)

		err = VerifyClusterGen(manifest.Environments{
			"env": manifest.EnvironmentDefinition{
				Name: "env",
				Type: manifest.Platform,
				Url: manifest.UrlDefinition{
					Type:  manifest.ValueUrlType,
					Name:  "URL",
					Value: server.URL + "/WRONG_URL",
				},
			},
		})
		assert.Error(t, err)

	})

}