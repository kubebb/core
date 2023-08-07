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
	"context"
	"errors"

	"github.com/kubebb/core/pkg/utils"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	ErrDecode            = errors.New("decode error")
	ErrCreatorChange     = errors.New("creator (spec.creator) should not change")
	ErrReleaseNameChange = errors.New("release name (spec.name) should not change")
	ErrComponentChange   = errors.New("component name and namespace (spec.component) should not change")
)

func getReqUserInfo(ctx context.Context) (authenticationv1.UserInfo, error) {
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return authenticationv1.UserInfo{}, err
	}
	return req.UserInfo, nil
}

func isSuperUser(u authenticationv1.UserInfo) bool {
	if u.Username == utils.GetOperatorUser() {
		return true
	}
	return slices.Contains(u.Groups, user.SystemPrivilegedGroup) || slices.Contains(u.Groups, serviceaccount.MakeNamespaceGroupName(metav1.NamespaceSystem))
}
