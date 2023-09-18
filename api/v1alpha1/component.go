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

package v1alpha1

import "github.com/Masterminds/semver/v3"

const (
	AddEventMsgTemplate    = "add new component %s"
	DelEventMsgTemplate    = "delete component %s"
	UpdateEventMsgTemplate = "update component %s. %d new,  %d deleted,  %d deprecated"
	OCIPullURLAnnotation   = Group + "/oci-pull-url"
	ValuesConfigMapLabel   = Group + "/component-name"
	ValuesConfigMapKey     = "values.yaml"
	ImagesConfigMapKey     = "images"
)

// ComponentVersionDiff When the version of a component changes,
// we need to give information about the event change,
// and we need to be clear about the versions that were added,
// the versions that were removed, and the versions that were deprecated.
func ComponentVersionDiff(o, n Component) ([]string, []string, []string) {
	var (
		added, deleted, deprecated []string

		nVersionSem, oVersionSem *semver.Version
	)

	i, j := 0, 0
	for i < len(n.Status.Versions) && j < len(o.Status.Versions) {
		// The version information comes from the helm repository, so we can ignore the error
		nVersionSem, _ = semver.NewVersion(n.Status.Versions[i].Version)
		oVersionSem, _ = semver.NewVersion(o.Status.Versions[j].Version)

		if nVersionSem.Equal(oVersionSem) {
			// there should be only from normal version to deprecated, not from deprecated that back to normal version.
			if n.Status.Versions[i].Deprecated && !o.Status.Versions[j].Deprecated {
				deprecated = append(deprecated, n.Status.Versions[i].Version)
			}
			i, j = i+1, j+1
			continue
		}

		if nVersionSem.GreaterThan(oVersionSem) {
			added = append(added, n.Status.Versions[i].Version)
			i++
			continue
		}

		deleted = append(deleted, o.Status.Versions[j].Version)
		j++
	}

	for ; j < len(o.Status.Versions); j++ {
		deleted = append(deleted, o.Status.Versions[j].Version)
	}
	for ; i < len(n.Status.Versions); i++ {
		added = append(added, n.Status.Versions[i].Version)
	}
	return added, deleted, deprecated
}

func GetComponentChartValuesConfigmapName(componentName, version string) string {
	return componentName + "-" + version
}
