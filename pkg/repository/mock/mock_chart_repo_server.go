/*
Copyright 2023 The Kubebb Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mock

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

var (
	// yaml1 only include chartmuseum with one version
	yaml1 = `apiVersion: v1
entries: 
  chartmuseum:
  - apiVersion: v2
    appVersion: 0.16.0
    created: "2023-07-27T07:49:45.023571147Z"
    description: Host your own Helm Chart Repository
    digest: 7d9181335a24d8907c2d0f3d72eb0aaf82831287273ccc250b80946d50577e7b
    home: https://github.com/helm/chartmuseum
    icon: https://raw.githubusercontent.com/chartmuseum/charts/main/logo.jpg
    keywords:
    - chartmuseum
    - helm
    - charts repo
    maintainers:
    - name: chartmuseum
      url: https://github.com/chartmuseum
    name: chartmuseum
    sources:
    - https://github.com/chartmuseum/charts/tree/main/src/chartmuseum
    - https://github.com/chartmuseum
    - https://github.com/helm/chartmuseum
    urls:
    - https://github.com/kubebb/components/releases/download/chartmuseum-3.10.1/chartmuseum-3.10.1.tgz
    version: 3.10.1
generated: "2023-09-11T08:26:00.296380064Z"`

	// yaml2 include two versions chartmuseum
	yaml2 = `apiVersion: v1
entries: 
  chartmuseum:
  - apiVersion: v2
    appVersion: 0.16.0
    created: "2023-08-14T08:36:07.186243368Z"
    description: Host your own Helm Chart Repository
    digest: 40d85cd21c5f49ed0b9961a457594fd43fd19e2bc1763dfb318e2a6fb8706ce4
    home: https://github.com/helm/chartmuseum
    icon: https://raw.githubusercontent.com/chartmuseum/charts/main/logo.jpg
    keywords:
    - chartmuseum
    - helm
    - charts repo
    maintainers:
    - name: chartmuseum
      url: https://github.com/chartmuseum
    name: chartmuseum
    sources:
    - https://github.com/chartmuseum/charts/tree/main/src/chartmuseum
    - https://github.com/chartmuseum
    - https://github.com/helm/chartmuseum
    urls:
    - https://github.com/kubebb/components/releases/download/chartmuseum-3.10.2/chartmuseum-3.10.2.tgz
    version: 3.10.2
  - apiVersion: v2
    appVersion: 0.16.0
    created: "2023-07-27T07:49:45.023571147Z"
    description: Host your own Helm Chart Repository
    digest: 7d9181335a24d8907c2d0f3d72eb0aaf82831287273ccc250b80946d50577e7b
    home: https://github.com/helm/chartmuseum
    icon: https://raw.githubusercontent.com/chartmuseum/charts/main/logo.jpg
    keywords:
    - chartmuseum
    - helm
    - charts repo
    maintainers:
    - name: chartmuseum
      url: https://github.com/chartmuseum
    name: chartmuseum
    sources:
    - https://github.com/chartmuseum/charts/tree/main/src/chartmuseum
    - https://github.com/chartmuseum
    - https://github.com/helm/chartmuseum
    urls:
    - https://github.com/kubebb/components/releases/download/chartmuseum-3.10.1/chartmuseum-3.10.1.tgz
    version: 3.10.1
generated: "2023-09-11T08:26:00.296380064Z"`

	// for testing filter, we will fitler chartmuseum, only remain minio
	yaml3 = `apiVersion: v1
entries: 
  chartmuseum:
  - apiVersion: v2
    appVersion: 0.16.0
    created: "2023-08-14T08:36:07.186243368Z"
    description: Host your own Helm Chart Repository
    digest: 40d85cd21c5f49ed0b9961a457594fd43fd19e2bc1763dfb318e2a6fb8706ce4
    home: https://github.com/helm/chartmuseum
    icon: https://raw.githubusercontent.com/chartmuseum/charts/main/logo.jpg
    keywords:
    - chartmuseum
    - helm
    - charts repo
    maintainers:
    - name: chartmuseum
      url: https://github.com/chartmuseum
    name: chartmuseum
    sources:
    - https://github.com/chartmuseum/charts/tree/main/src/chartmuseum
    - https://github.com/chartmuseum
    - https://github.com/helm/chartmuseum
    urls:
    - https://github.com/kubebb/components/releases/download/chartmuseum-3.10.2/chartmuseum-3.10.2.tgz
    version: 3.10.2
  - apiVersion: v2
    appVersion: 0.16.0
    created: "2023-07-27T07:49:45.023571147Z"
    description: Host your own Helm Chart Repository
    digest: 7d9181335a24d8907c2d0f3d72eb0aaf82831287273ccc250b80946d50577e7b
    home: https://github.com/helm/chartmuseum
    icon: https://raw.githubusercontent.com/chartmuseum/charts/main/logo.jpg
    keywords:
    - chartmuseum
    - helm
    - charts repo
    maintainers:
    - name: chartmuseum
      url: https://github.com/chartmuseum
    name: chartmuseum
    sources:
    - https://github.com/chartmuseum/charts/tree/main/src/chartmuseum
    - https://github.com/chartmuseum
    - https://github.com/helm/chartmuseum
    urls:
    - https://github.com/kubebb/components/releases/download/chartmuseum-3.10.1/chartmuseum-3.10.1.tgz
    version: 3.10.1
  minio:
  - apiVersion: v1
    appVersion: RELEASE.2023-02-10T18-48-39Z
    created: "2023-08-16T04:27:42.388058016Z"
    description: Multi-Cloud Object Storage
    digest: 3d67a50567bff4ee9ea71663f169f658bf23223da77d064b0c0668f47c422e0e
    home: https://min.io
    icon: https://min.io/resources/img/logo/MINIO_wordmark.png
    keywords:
    - minio
    - storage
    - object-storage
    - s3
    - cluster
    maintainers:
    - email: dev@minio.io
      name: MinIO, Inc
    name: minio
    sources:
    - https://github.com/minio/minio
    urls:
    - https://github.com/kubebb/components/releases/download/minio-5.0.8/minio-5.0.8.tgz
    version: 5.0.8
  - apiVersion: v1
    appVersion: RELEASE.2023-02-10T18-48-39Z
    created: "2023-07-03T09:03:55.998503794Z"
    description: Multi-Cloud Object Storage
    digest: cf3d46b8abeb363e7913d907c243071109d8738dfdb30a7236c9f082c7bd29ba
    home: https://min.io
    icon: https://min.io/resources/img/logo/MINIO_wordmark.png
    deprecated: true
    keywords:
    - minio
    - storage
    - object-storage
    - s3
    - cluster
    maintainers:
    - email: dev@minio.io
      name: MinIO, Inc
    name: minio
    sources:
    - https://github.com/minio/minio
    urls:
    - https://github.com/kubebb/components/releases/download/minio-5.0.7/minio-5.0.7.tgz
    version: 5.0.7
generated: "2023-09-11T10:26:00.296380064Z"`
)

func NewMockChartServer(port int) *http.Server {
	srv := &http.Server{
		Addr: fmt.Sprintf("127.0.0.1:%d", port),
	}
	now := time.Now()
	http.HandleFunc("/a/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		diff := time.Since(now).Seconds()
		if diff >= 120 {
			// Simulate adding a new chart package
			_, _ = w.Write([]byte(yaml2))
			return
		}
		_, _ = w.Write([]byte(yaml1))
	})
	http.HandleFunc("/b/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte(yaml3))
	})
	srv.Handler = http.DefaultServeMux
	return srv
}

func NewMockChartServerWithBasicAuth(port int, username, password string) *http.Server {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}
	uu := base64.StdEncoding.EncodeToString([]byte(username))
	pp := base64.StdEncoding.EncodeToString([]byte(password))
	now := time.Now()
	http.HandleFunc("/a/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != uu || p != pp {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized"))
			return
		}

		w.Header().Set("Content-Type", "text/yaml")
		diff := time.Since(now).Seconds()
		if diff >= 120 {
			// Simulate adding a new chart package
			_, _ = w.Write([]byte(yaml2))
			return
		}
		_, _ = w.Write([]byte(yaml1))
	})
	http.HandleFunc("/b/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != uu || p != pp {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unaut=orized"))
			return
		}

		w.Header().Set("Content-Type", "text/yaml")
		_, _ = w.Write([]byte(yaml3))
	})

	srv.Handler = http.DefaultServeMux
	return srv
}
