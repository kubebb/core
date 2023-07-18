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

import (
	"reflect"
	"testing"
)

func TestComponentVersionDiff(t *testing.T) {
	type input struct {
		name                                string
		o, n                                Component
		expAdded, expDeleted, expDeprecated []string
	}
	testCases := []input{
		{
			name: "there are no changes to the version",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
		},

		{
			name: "no new versions, but one version is deprecated",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2"},
						{Version: "1", Deprecated: true},
					},
				},
			},
			expDeprecated: []string{"1"},
		},

		{
			name: "add a new version",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "3"},
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			expAdded: []string{"3"},
		},

		{
			name: "add a new version and one version is deprecated",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "3"},
						{Version: "2"},
						{Version: "1", Deprecated: true},
					},
				},
			},
			expAdded:      []string{"3"},
			expDeprecated: []string{"1"},
		},

		{
			name: "remove one version",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2"},
					},
				},
			},
			expDeleted: []string{"1"},
		},

		{
			name: "remove one version and one version is deprecated",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "3"},
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "2", Deprecated: true},
						{Version: "1"},
					},
				},
			},
			expDeleted:    []string{"3"},
			expDeprecated: []string{"2"},
		},

		{
			name: "add several versions, remove several versions, deprecate several versions",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "3"},
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "5"},
						{Version: "4"},
						{Version: "2", Deprecated: true},
					},
				},
			},
			expAdded:      []string{"5", "4"},
			expDeleted:    []string{"3", "1"},
			expDeprecated: []string{"2"},
		},

		{
			name: "there is no overlap between the versions",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "3"},
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "5"},
						{Version: "4"},
					},
				},
			},
			expAdded:   []string{"5", "4"},
			expDeleted: []string{"3", "2", "1"},
		},

		{
			name: "add completely",
			o: Component{
				Status: ComponentStatus{},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "5"},
						{Version: "4"},
					},
				},
			},
			expAdded: []string{"5", "4"},
		},

		{
			name: "delete completely",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "3"},
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{},
			},
			expDeleted: []string{"3", "2", "1"},
		},

		{
			name: "discard completely",
			o: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "3"},
						{Version: "2"},
						{Version: "1"},
					},
				},
			},
			n: Component{
				Status: ComponentStatus{
					Versions: []ComponentVersion{
						{Version: "3", Deprecated: true},
						{Version: "2", Deprecated: true},
						{Version: "1", Deprecated: true},
					},
				},
			},
			expDeprecated: []string{"3", "2", "1"},
		},
	}
	for _, tc := range testCases {
		added, deleted, deprecated := ComponentVersionDiff(tc.o, tc.n)
		if !reflect.DeepEqual(added, tc.expAdded) {
			t.Fatalf("%s expect %v get %v\n", tc.name, tc.expAdded, added)
		}
		if !reflect.DeepEqual(deleted, tc.expDeleted) {
			t.Fatalf("%s expect %v get %v\n", tc.name, tc.expDeleted, deleted)
		}
		if !reflect.DeepEqual(deprecated, tc.expDeprecated) {
			t.Fatalf("%s expect %v get %v\n", tc.name, tc.expDeprecated, deprecated)
		}
	}
}
