#!/bin/bash
#
# Copyright contributors to the Kubebb Core project
#
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at:
#
# 	  http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
if [[ $RUNNER_DEBUG -eq 1 ]] || [[ $GITHUB_RUN_ATTEMPT -gt 1 ]]; then
	# use [debug logging](https://docs.github.com/en/actions/monitoring-and-troubleshooting-workflows/enabling-debug-logging)
	# or run the same test multiple times.
	set -x
fi
export TERM=xterm-color

KindName="kind"
TimeoutSeconds=${TimeoutSeconds:-"600"}
HelmTimeout=${HelmTimeout:-"1800s"}
KindVersion=${KindVersion:-"v1.24.4"}
TempFilePath=${TempFilePath:-"/tmp/kubebb-core-example-test"}
KindConfigPath=${TempFilePath}/kind-config.yaml
InstallDirPath=${TempFilePath}/building-base
DefaultPassWord=${DefaultPassWord:-'passw0rd'}
LOG_DIR=${LOG_DIR:-"/tmp/kubebb-core-example-test/logs"}
RootPath=$(dirname -- "$(readlink -f -- "$0")")/../..

Timeout="${TimeoutSeconds}s"
mkdir ${TempFilePath} || true

function debugInfo {
	if [[ $? -eq 0 ]]; then
		exit 0
	fi
	if [[ $debug -ne 0 ]]; then
		exit 1
	fi

	warning "debugInfo start 🧐"
	mkdir -p $LOG_DIR

	warning "1. Try to get all resources "
	kubectl api-resources --verbs=list -o name | xargs -n 1 kubectl get -A --ignore-not-found=true --show-kind=true >$LOG_DIR/get-all-resources-list.log
	kubectl api-resources --verbs=list -o name | xargs -n 1 kubectl get -A -oyaml --ignore-not-found=true --show-kind=true >$LOG_DIR/get-all-resources-yaml.log

	warning "2. Try to describe all resources "
	kubectl api-resources --verbs=list -o name | xargs -n 1 kubectl describe -A >$LOG_DIR/describe-all-resources.log

	warning "3. Try to export kind logs to $LOG_DIR..."
	kind export logs --name=${KindName} $LOG_DIR
	sudo chown -R $USER:$USER $LOG_DIR

	warning "debugInfo finished ! "
	warning "This means that some tests have failed. Please check the log. 🌚"
	debug=1
	exit 1
}
trap 'debugInfo $LINENO' ERR
trap 'debugInfo $LINENO' EXIT
debug=0

function cecho() {
	declare -A colors
	colors=(
		['black']='\E[0;47m'
		['red']='\E[0;31m'
		['green']='\E[0;32m'
		['yellow']='\E[0;33m'
		['blue']='\E[0;34m'
		['magenta']='\E[0;35m'
		['cyan']='\E[0;36m'
		['white']='\E[0;37m'
	)
	local defaultMSG="No message passed."
	local defaultColor="black"
	local defaultNewLine=true
	while [[ $# -gt 1 ]]; do
		key="$1"
		case $key in
		-c | --color)
			color="$2"
			shift
			;;
		-n | --noline)
			newLine=false
			;;
		*)
			# unknown option
			;;
		esac
		shift
	done
	message=${1:-$defaultMSG}     # Defaults to default message.
	color=${color:-$defaultColor} # Defaults to default color, if not specified.
	newLine=${newLine:-$defaultNewLine}
	echo -en "${colors[$color]}"
	echo -en "$message"
	if [ "$newLine" = true ]; then
		echo
	fi
	tput sgr0 #  Reset text attributes to normal without clearing screen.
	return
}

function warning() {
	cecho -c 'yellow' "$@"
}

function error() {
	cecho -c 'red' "$@"
}

function info() {
	cecho -c 'blue' "$@"
}

function waitComponentSatus() {
	namespace=$1
	componentName=$2
	START_TIME=$(date +%s)
	while true; do
		versions=$(kubectl -n${namespace} get components.core.kubebb.k8s.com.cn ${componentName} -ojson | jq -r '.status.versions|length')
		if [[ $versions -ne 0 ]]; then
			echo "component ${componentName} already have version information and can be installed"
			break
		fi
		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			exit 1
		fi
		sleep 5
	done
}

function waitComponentPlanDone() {
	namesapce=$1
	componentPlanName=$2
	START_TIME=$(date +%s)
	while true; do
		doneConds=$(kubectl -n${namespace} get ComponentPlan ${componentPlanName} -ojson | jq -r '.status.conditions' | jq 'map(select(.type == "Succeeded"))|map(select(.status == "True"))|length')
		if [[ $doneConds -ne 0 ]]; then
			echo "componentPlan ${componentPlanName} done"
			break
		fi

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			exit 1
		fi
		sleep 5
	done
}

info "1. create kind cluster"
git clone https://github.com/kubebb/building-base.git ${InstallDirPath}
cd ${InstallDirPath}
export IGNORE_FIXED_IMAGE_LOAD=YES
. ./scripts/kind.sh

info "2. install kubebb core"

info "2.1 Add kubebb chart repository"
helm repo add kubebb https://kubebb.github.io/components
helm repo update

info "2.2 search kubebb"
search_result=$(helm search repo kubebb/kubebb)
if [[ $search_result == "No results found" ]]; then
	error "not found chart kubebb/kubebb"
	exit 1
fi

info "2.3 intall kubebb release kubebb in namesapce kubebb-system"
info "2.3.1 create namespace kubebb-system"
kubectl create ns kubebb-system

info "2.3.2 create kubebb release"
helm -nkubebb-system install kubebb kubebb/kubebb --wait

cd ${RootPath}
info "3 test nginx by doing once componentPlan"
info "3.1 create bitnami repository"
kubectl apply -f config/samples/core_v1alpha1_repository_bitnami.yaml
waitComponentSatus "kubebb-system" "repository-bitnami-sample.nginx"

info "3.2 create nginx componentplan"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml
waitComponentPlanDone "kubebb-system" "do-once-nginx-sample-15.0.2"

info "all finished! ✅"
