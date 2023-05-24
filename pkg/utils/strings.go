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

package utils

func AddString(finalizers []string, f string) []string {
	for _, f1 := range finalizers {
		if f1 == f {
			return finalizers
		}
	}
	return append(finalizers, f)
}

func RemoveString(finalziers []string, f string) []string {
	index := 0
	for idx := 0; idx < len(finalziers); idx++ {
		if finalziers[idx] == f {
			continue
		}
		finalziers[index] = finalziers[idx]
		index++
	}
	return finalziers[:index]
}

func ContainString(finalizers []string, f string) bool {
	for _, f1 := range finalizers {
		if f1 == f {
			return true
		}
	}
	return false
}
