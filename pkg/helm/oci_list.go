/*
 * Copyright 2023 The Kubebb Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/goharbor/go-client/pkg/harbor"
	harborrepository "github.com/goharbor/go-client/pkg/sdk/v2.0/client/repository"
	"github.com/google/go-github/v54/github"
	"golang.org/x/oauth2"
	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/utils/env"

	corev1alpha1 "github.com/kubebb/core/api/v1alpha1"
)

// GetOCIRepoList retrieves the OCI packages repository based on the given path and repository.
// TODO need add auth for private repo
func GetOCIRepoList(ctx context.Context, repo *corev1alpha1.Repository) ([]string, error) {
	if !repo.IsOCI() {
		return nil, nil
	}
	parse, err := url.Parse(repo.Spec.URL)
	if err != nil {
		return nil, err
	}
	parse.Scheme = "https"
	switch parse.Host {
	case "docker.io", "registry-1.docker.io":
		return GetDockerhubHelmRepository(ctx, repo)
	case "ghcr.io":
		return GetGithubHelmRepository(ctx, repo)
	default:
		return GetHarborRepository(ctx, repo)
	}
}

// GetHarborRepository retrieves the Harbor packages repository based on the given path and repository.
func GetHarborRepository(ctx context.Context, repo *corev1alpha1.Repository) ([]string, error) {
	parse, err := url.Parse(repo.Spec.URL)
	if err != nil {
		return nil, err
	}
	parse.Scheme = "https"
	p := strings.Split(parse.Path, "/")
	if len(p) < 2 {
		return nil, fmt.Errorf("invalid url for github:%s in repo:%s/%s", repo.Spec.URL, repo.GetNamespace(), repo.GetName())
	}
	if len(p) == 3 {
		//	/helm-test/nginx
		return []string{repo.Spec.URL}, nil
	}
	cs := &harbor.ClientSetConfig{
		URL:      parse.Scheme + "://" + parse.Host,
		Insecure: false, // TODO
		Username: "",
		Password: "",
	}
	client, err := harbor.NewClientSet(cs)
	if err != nil {
		return nil, err
	}
	param := harborrepository.NewListRepositoriesParams().WithDefaults()
	param.ProjectName = p[1]
	res := []string{}
	sum := int64(len(res))
	total := *param.PageSize
	for sum < total {
		projects, err := client.V2().Repository.ListRepositories(ctx, param)
		if err != nil {
			return nil, err
		}
		for _, p := range projects.Payload {
			res = append(res, p.Name)
		}
		sum = int64(len(res))
		total = projects.XTotalCount
		*param.Page++
	}
	for i := 0; i < len(res); i++ {
		res[i] = repo.Spec.URL + "/" + strings.TrimPrefix(res[i], p[1]+"/")
	}
	return res, nil
}

// GetGithubHelmRepository retrieves the GitHub packages repository based on the given path and repository.
func GetGithubHelmRepository(ctx context.Context, repo *corev1alpha1.Repository) ([]string, error) {
	parse, err := url.Parse(repo.Spec.URL)
	if err != nil {
		return nil, err
	}
	parse.Scheme = "https"
	// for orgs /kubesphere
	// for common repo /jimcronqvist/helm-charts
	// Note: because github api has no support for check package manifest
	p := strings.Split(parse.Path, "/")
	if len(p) < 2 { // /
		return nil, fmt.Errorf("invalid url for Github:%s in repo:%s/%s", repo.Spec.URL, repo.GetNamespace(), repo.GetName())
	}
	var repositoryPrefix string
	if len(p) == 3 { // /Abirdcfly/helm-oci-example
		repositoryPrefix = p[2] + "/"
	}
	if len(p) == 4 { // /Abirdcfly/helm-oci-example/nginx
		return []string{repo.Spec.URL}, nil
	}
	defaultValue := "hyaTcp6MWDa5I1LmdFRjsIeshwTNCq22G"                   // Use string join to avoid GitHub warning...
	token := env.GetString("GITHUB_PAT_TOKEN", "ghp_"+defaultValue+"Ung") // the default token with no expiration and has only read:packages permission
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	get, _, err := client.Users.Get(ctx, p[1])
	if err != nil {
		return nil, err
	}
	var listPackages func(ctx context.Context, user string, opts *github.PackageListOptions) ([]*github.Package, *github.Response, error)
	switch get.GetType() {
	case "User":
		listPackages = client.Users.ListPackages
	case "Organization":
		listPackages = client.Organizations.ListPackages
	default:
		return nil, fmt.Errorf("cant get %s type", p[1])
	}
	name := p[1]
	page := 1
	packages, resp, err := githubGetPage(ctx, listPackages, name, page)
	if err != nil {
		return nil, err
	}
	allpackages := packages
	for resp.NextPage != 0 {
		page++
		packages, resp, err = githubGetPage(ctx, listPackages, name, page)
		if err != nil {
			return nil, err
		}
		allpackages = append(allpackages, packages...)
	}
	var filterPackage []string
	for _, p := range packages {
		if repositoryPrefix != "" { // helm-oci-example/
			// oci://ghcr.io/abirdcfly/helm-oci-example/nginx will keep, but drop oci://ghcr.io/abirdcfly/nginx
			// use helm-oci-example not helm-oci-example/, Because in one case, helm-oci-eample is the name of a separate image, not the name of a repository
			if strings.HasPrefix(p.GetName(), repositoryPrefix[:len(repositoryPrefix)-1]) {
				filterPackage = append(filterPackage, p.GetName())
			}
		} else {
			filterPackage = append(filterPackage, p.GetName())
		}
	}
	for i := 0; i < len(filterPackage); i++ {
		// filterPackage[i]:redis url: oci://ghcr.io/abirdcfly/redis
		if filterPackage[i]+"/" == repositoryPrefix {
			filterPackage[i] = repo.Spec.URL
			continue
		}
		filterPackage[i] = repo.Spec.URL + "/" + strings.TrimPrefix(filterPackage[i], repositoryPrefix)
	}
	return filterPackage, nil
}

func githubGetPage(ctx context.Context, listPackages func(ctx context.Context, user string, opts *github.PackageListOptions) ([]*github.Package, *github.Response, error), name string, page int) (packages []*github.Package, resp *github.Response, err error) {
	packages, resp, err = listPackages(ctx, name, &github.PackageListOptions{
		Visibility:  github.String("public"),
		PackageType: github.String("container"),
		State:       github.String("active"),
		ListOptions: github.ListOptions{
			Page:    page,
			PerPage: 100,
		},
	})
	if err != nil {
		return
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode == http.StatusOK {
		return
	}
	err = fmt.Errorf("bad status code %q", resp.Status)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var buf []byte
		buf, err = io.ReadAll(resp.Body)
		if err == nil {
			err = fmt.Errorf("bad status code %q: %s", resp.Status, string(buf))
			return
		}
		return
	}
	return
}

// GetDockerhubHelmRepository retrieves the Dockerhub Helm repository based on the given path and repository.
func GetDockerhubHelmRepository(ctx context.Context, repo *corev1alpha1.Repository) ([]string, error) {
	parse, err := url.Parse(repo.Spec.URL)
	if err != nil {
		return nil, err
	}
	parse.Scheme = "https"
	p := strings.Split(parse.Path, "/")
	if len(p) == 3 && p[0] == "" { // /bitnamicharts/nginx
		return []string{repo.Spec.URL}, nil
	}
	if len(p) != 2 && p[0] != "" {
		return nil, fmt.Errorf("invalid url for dockerhub:%s in repo:%s/%s", repo.Spec.URL, repo.GetNamespace(), repo.GetName())
	}

	baseURL := "https://hub.docker.com/v2/repositories/" + p[1]
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("page_size", "100")
	q.Add("page", "1")
	q.Add("ordering", "last_updated")
	u.RawQuery = q.Encode()

	repos, err := dockerhubGetPage(u.String())
	if err != nil {
		return nil, err
	}
	res := filterOCIDockerhubRepositoryResult(repos.Results)
	for next := repos.Next; next != ""; {
		n, err := dockerhubGetPage(repos.Next)
		if err != nil {
			return nil, err
		}
		res = append(res, filterOCIDockerhubRepositoryResult(n.Results)...)
		next = n.Next
	}
	for i := 0; i < len(res); i++ {
		res[i] = repo.Spec.URL + "/" + strings.TrimPrefix(res[i], p[1]+"/")
	}
	return res, nil
}

func filterOCIDockerhubRepositoryResult(raw []dockerhubRepositoryResult) (res []string) {
	for _, r := range raw {
		isOCI := false
		for _, c := range r.ContentTypes {
			if c == "helm" {
				isOCI = true
			}
		}
		for _, c := range r.MediaTypes {
			if c == registry.ConfigMediaType {
				isOCI = true
			}
		}
		if isOCI {
			res = append(res, r.Name)
		}
	}
	return res
}
func dockerhubGetPage(url string) (*dockerhubRepositoryResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header["Accept"] = []string{"application/json"}
	req.Header["Content-Type"] = []string{"application/json"}
	req.Header["User-Agent"] = []string{"hub-tool/v0.4.5"}
	// req.Header["Authorization"] = []string{fmt.Sprintf("Bearer %s", token)} #TODO dockerhub use jwt token
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close() //nolint:errcheck
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("bad status code %q", resp.Status)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("bad status code %q", resp.Status)
		}
		buf, err := io.ReadAll(resp.Body)
		if err == nil {
			var responseBody map[string]string
			if err := json.Unmarshal(buf, &responseBody); err == nil {
				for _, k := range []string{"message", "detail"} {
					if msg, ok := responseBody[k]; ok {
						return nil, fmt.Errorf(msg)
					}
				}
			}
		}
		return nil, fmt.Errorf("bad status code %q", resp.Status)
	}
	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bad status code %q: %s", resp.Status, string(buf))
	}
	hubResponse := &dockerhubRepositoryResponse{}
	if err := json.Unmarshal(buf, hubResponse); err != nil {
		return nil, err
	}
	return hubResponse, nil
}

// inspire by https://github.com/docker/hub-tool/blob/04791d1b4169d219fd4d867137e507a2113334d9/pkg/hub/repositories.go#L120
type dockerhubRepositoryResponse struct {
	Count    int                         `json:"count"`
	Next     string                      `json:"next,omitempty"`
	Previous string                      `json:"previous,omitempty"`
	Results  []dockerhubRepositoryResult `json:"results,omitempty"`
}

// inspire by https://github.com/docker/hub-tool/blob/04791d1b4169d219fd4d867137e507a2113334d9/pkg/hub/repositories.go#L120
// And add missing items (like results.media_types) according to the results of the actual request to https://hub.docker.com/v2/repositories/xxx
// This API does not appear in [dockerhub's API documentation](https://docs.docker.com/docker-hub/api/latest/), so presumably it may change in the future.
type dockerhubRepositoryResult struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	RepositoryType    string    `json:"repository_type"`
	Status            int       `json:"status"`
	StatusDescription string    `json:"status_description"`
	Description       string    `json:"description,omitempty"`
	IsPrivate         bool      `json:"is_private"`
	StarCount         int       `json:"star_count"`
	PullCount         int       `json:"pull_count"`
	LastUpdated       time.Time `json:"last_updated"`
	DateRegistered    string    `json:"date_registered"`
	Affiliation       string    `json:"affiliation"`
	MediaTypes        []string  `json:"media_types"`
	ContentTypes      []string  `json:"content_types"`
	// The following items seems to require a specific request to trigger.
	CanEdit     bool   `json:"can_edit"`
	IsAutomated bool   `json:"is_automated"`
	IsMigrated  bool   `json:"is_migrated"`
	User        string `json:"user"`
}
