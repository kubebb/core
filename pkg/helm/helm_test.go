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

import "testing"

func TestParseDescription(t *testing.T) {
	type args struct {
		desc string
	}
	tests := []struct {
		name           string
		args           args
		wantNs         string
		wantName       string
		wantUID        string
		wantGeneration int64
		wantRaw        string
	}{
		{
			name: "normal",
			args: args{
				desc: "core:kubebb-system/nginx/894d23c7-a177-4e6c-9d5c-bd8efe1e6df8/1 description",
			},
			wantNs:         "kubebb-system",
			wantName:       "nginx",
			wantUID:        "894d23c7-a177-4e6c-9d5c-bd8efe1e6df8",
			wantGeneration: 1,
			wantRaw:        "description",
		},
		{
			name: "empty description",
			args: args{
				desc: "core:kubebb-system/nginx/894d23c7-a177-4e6c-9d5c-bd8efe1e6df8/1 ",
			},
			wantNs:         "kubebb-system",
			wantName:       "nginx",
			wantUID:        "894d23c7-a177-4e6c-9d5c-bd8efe1e6df8",
			wantGeneration: 1,
			wantRaw:        "",
		},
		{
			name: "empty",
			args: args{
				desc: "",
			},
			wantNs:         "",
			wantName:       "",
			wantUID:        "",
			wantGeneration: 0,
			wantRaw:        "",
		},
		{
			name: "raw description",
			args: args{
				desc: "balabala xxxx",
			},
			wantNs:         "",
			wantName:       "",
			wantUID:        "",
			wantGeneration: 0,
			wantRaw:        "balabala xxxx",
		},
		{
			name: "raw description with prefix core",
			args: args{
				desc: "core:balabala xxxx",
			},
			wantNs:         "",
			wantName:       "",
			wantUID:        "",
			wantGeneration: 0,
			wantRaw:        "core:balabala xxxx",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNs, gotName, gotUID, gotGeneration, gotRaw := ParseDescription(tt.args.desc)
			if gotNs != tt.wantNs {
				t.Errorf("ParseDescription() gotNs = %v, want %v", gotNs, tt.wantNs)
			}
			if gotName != tt.wantName {
				t.Errorf("ParseDescription() gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotUID != tt.wantUID {
				t.Errorf("ParseDescription() gotUID = %v, want %v", gotUID, tt.wantUID)
			}
			if gotGeneration != tt.wantGeneration {
				t.Errorf("ParseDescription() gotGeneration = %v, want %v", gotGeneration, tt.wantGeneration)
			}
			if gotRaw != tt.wantRaw {
				t.Errorf("ParseDescription() gotRaw = %v, want %v", gotRaw, tt.wantRaw)
			}
		})
	}
}
