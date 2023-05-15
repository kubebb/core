#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ROOT_PATH=$(dirname "${BASH_SOURCE[0]}")/..
source "${ROOT_PATH}/hack/lib/init.sh"

go::setup_env
cd "${ROOT_PATH}"

echo "codegen start."
echo "tools install..."

GO111MODULE=on go install k8s.io/code-generator/cmd/deepcopy-gen@v0.24.2

echo "Generating with deepcopy-gen..."
deepcopy-gen \
	--go-header-file hack/boilerplate/boilerplate.generatego.txt \
	--input-dirs=github.com/kubebb/core/apis/v1alpha1 \
	--output-package=github.com/kubebb/core/apis/v1alpha1 \
	--output-file-base=zz_generated.deepcopy

echo "codegen done."
