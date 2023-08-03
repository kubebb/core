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
TimeoutSeconds=${TimeoutSeconds:-"300"}
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

function waitComponentStatus() {
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
	namespace=$1
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
			kubectl -n${namespace} get ComponentPlan -o yaml
			exit 1
		fi
		sleep 5
	done
}

function waitPodReady() {
	namespace=$1
	podLabel=$2
	START_TIME=$(date +%s)
	while true; do
		readStatus=$(kubectl -n${namespace} get po -l ${podLabel} --ignore-not-found=true -o json | jq -r '.items[0].status.conditions[] | select(."type"=="Ready") | .status')
		if [[ $readStatus == "True" ]]; then
			echo "Pod ${podLabel} ready"
			kubectl -n${namespace} get po -l ${podLabel}
			break
		fi

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			kubectl describe po -n${namespace} -l ${podLabel}
			kubectl get po -n${namespace} --show-labels
			exit 1
		fi
		sleep 5
	done
}

function deleteComponentPlan() {
	namespace=$1
	componentPlanName=$2
	START_TIME=$(date +%s)
	while true; do
		kubectl -n${namespace} delete ComponentPlan ${componentPlanName} --wait
		if [[ $? -eq 0 ]]; then
			echo "delete componentPlan ${componentPlanName} done"
			break
		fi

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			exit 1
		fi
		sleep 30
	done
	while true; do
		results=$(helm status -n ${namespace} ${componentPlanName})
		if [[ $results == "" ]]; then
			echo "helm status also show ${componentPlanName} removed"
			break
		fi

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			helm list -A -a
			helm status -n ${namespace} ${componentPlanName}
			exit 1
		fi
		sleep 30
	done
}

function getPodImage() {
	namespace=$1
	podLabel=$2
	want=$3
	START_TIME=$(date +%s)
	while true; do
		images=$(kubectl -n${namespace} get po -l ${podLabel} --ignore-not-found=true -o json | jq -r '.items[0].status.containerStatuses[].image')
		if [[ $images =~ $want ]]; then
			echo "$want found."
			break
		fi

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			kubectl get po -n${namespace} -l ${podLabel} -o yaml
			kubectl get po -n${namespace} --show-labels
			echo $images
			exit 1
		fi
		sleep 5
	done
}

function getHelmRevision() {
	namespace=$1
	releaseName=$2
	wantRevision=$3
	START_TIME=$(date +%s)
	while true; do
		get=$(helm status -n ${namespace} ${releaseName} -o json | jq '.version')
		if [[ $get == $wantRevision ]]; then
			echo "${releaseName} revision:${wantRevision} found."
			break
		fi
		echo "${releaseName} revision:${get} found.but want:${wantRevision}"

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			helm list -A -a
			exit 1
		fi
		sleep 5
	done
}

function getDeployReplicas() {
	namespace=$1
	deployName=$2
	want=$3
	START_TIME=$(date +%s)
	while true; do
		images=$(kubectl -n${namespace} get deploy ${deployName} --ignore-not-found=true -o json | jq -r '.spec.replicas')
		if [[ $images == $want ]]; then
			echo "replicas $want found."
			break
		fi

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			kubectl describe deploy -n${namespace} ${deployName}
			exit 1
		fi
		sleep 5
	done
}

function validateComponentPlanStatusLatestValue() {
	namespace=$1
	componentPlanName=$2
	want=$3
	latestValue=$(kubectl -n${namespace} get ComponentPlan ${componentPlanName} -ojson | jq -r '.status.latest')
	if [[ $latestValue != $want ]]; then
		echo "componentPlan ${componentPlanName} status.latest is $latestValue, not $want"
		kubectl -n${namespace} get ComponentPlan ${componentPlanName} -o yaml
		exit 1
		break
	fi
}

function waitComponentPlanRetryTime() {
	namespace=$1
	componentPlanName=$2
	retryTimeWant=$3
	START_TIME=$(date +%s)
	while true; do
		anno=$(kubectl -n${namespace} get ComponentPlan ${componentPlanName} -ojson | jq -r '.metadata.annotations["core.kubebb.k8s.com.cn/componentplan-retry"]')
		if [[ $anno == $retryTimeWant ]]; then
			echo "componentPlan ${componentPlanName} retry time match"
			break
		fi

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			kubectl -n${namespace} get ComponentPlan -o yaml
			exit 1
		fi
		sleep 5
	done
}

info "1. create kind cluster"
git clone https://github.com/kubebb/building-base.git ${InstallDirPath} || true
cd ${InstallDirPath}
export IGNORE_FIXED_IMAGE_LOAD=YES
. ./scripts/kind.sh

info "2. install kubebb core"
info "2.1 Add kubebb chart repository"
helm repo add kubebb https://kubebb.github.io/components
helm repo update kubebb

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
docker tag kubebb/core:latest kubebb/core:example-e2e
kind load docker-image kubebb/core:example-e2e
helm -nkubebb-system install kubebb kubebb/kubebb --set deployment.image=kubebb/core:example-e2e --wait
cd ${RootPath}
kubectl kustomize config/crd | kubectl apply -f -

info "3 try to verify that the common steps are valid"
info "3.1 create bitnami repository"
kubectl apply -f config/samples/core_v1alpha1_repository_bitnami.yaml
waitComponentStatus "kubebb-system" "repository-bitnami-sample.nginx"

info "3.2 create nginx componentplan"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml
waitComponentPlanDone "kubebb-system" "do-once-nginx-sample-15.0.2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getHelmRevision "kubebb-system" "my-nginx" "1"
deleteComponentPlan "kubebb-system" "do-once-nginx-sample-15.0.2"

info "3.3 create nginx-15.0.2 componentplan to verify imageOverride in componentPlan is valid"
kubectl apply -f config/samples/core_v1alpha1_componentplan_image_override.yaml
waitComponentPlanDone "kubebb-system" "nginx-15.0.2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getPodImage "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2" "docker.io/bitnami/nginx:latest"
getHelmRevision "kubebb-system" "my-nginx" "1"
deleteComponentPlan "kubebb-system" "nginx-15.0.2"

info "3.4 create nginx-replicas-example-1/2 componentplan to verify value override in componentPlan is valid"
kubectl apply -f config/samples/core_v1alpha1_nginx_replicas2_componentplan.yaml
waitComponentPlanDone "kubebb-system" "nginx-replicas-example-1"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx-replicas-example-1,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getDeployReplicas "kubebb-system" "my-nginx-replicas-example-1" "2"
getHelmRevision "kubebb-system" "my-nginx-replicas-example-1" "1"
deleteComponentPlan "kubebb-system" "nginx-replicas-example-1"

waitComponentPlanDone "kubebb-system" "nginx-replicas-example-2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx-replicas-example-2,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getDeployReplicas "kubebb-system" "my-nginx-replicas-example-2" "2"
getHelmRevision "kubebb-system" "my-nginx-replicas-example-2" "1"
deleteComponentPlan "kubebb-system" "nginx-replicas-example-2"

info "4 try to verify that the repository imageOverride steps are valid"
info "4.1 create repository-grafana-sample-image repository"
kubectl apply -f config/samples/core_v1alpha1_repository_grafana_image_repo_override.yaml
waitComponentStatus "kubebb-system" "repository-grafana-sample-image.grafana"

info "4.2 create grafana subscription"
kubectl apply -f config/samples/core_v1alpha1_grafana_subscription.yaml
getPodImage "kubebb-system" "app.kubernetes.io/instance=grafana-sample,app.kubernetes.io/name=grafana" "192.168.1.1:5000/grafana-local/grafana"
getHelmRevision "kubebb-system" "grafana-sample" "1"
kubectl delete -f config/samples/core_v1alpha1_grafana_subscription.yaml

info "5 try to verify that common use of componentPlan are valid"
info "5.1 create componentPlan do-once-nginx-sample-15.0.2"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml
waitComponentPlanDone "kubebb-system" "do-once-nginx-sample-15.0.2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getHelmRevision "kubebb-system" "my-nginx" "1"

info "5.2 update componentPlan do-once-nginx-sample-15.0.2 with update replicaCount to 2"
kubectl patch componentplan -n kubebb-system do-once-nginx-sample-15.0.2 --type='json' \
	-p='[{"op": "replace", "path": "/spec/override", "value": {"values": {"replicaCount": 2}}}]'
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getHelmRevision "kubebb-system" "my-nginx" "2"
getDeployReplicas "kubebb-system" "my-nginx" "2"

info "5.3 create new componentPlan to update nginx version"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan_15.1.0.yaml
waitComponentPlanDone "kubebb-system" "do-once-nginx-sample-15.1.0"
validateComponentPlanStatusLatestValue "kubebb-system" "do-once-nginx-sample-15.1.0" "true"
validateComponentPlanStatusLatestValue "kubebb-system" "do-once-nginx-sample-15.0.2" "false"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.1.0"
getHelmRevision "kubebb-system" "my-nginx" "3"
deleteComponentPlan "kubebb-system" "do-once-nginx-sample-15.1.0"
deleteComponentPlan "kubebb-system" "do-once-nginx-sample-15.0.2"

info "5.4 Verify long running componentPlan install don not block others to install"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan_long_ready.yaml
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml --dry-run -o json | jq '.spec.name="my-nginx-back"' | kubectl apply -f -
waitComponentPlanDone "kubebb-system" "do-once-nginx-sample-15.0.2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx-back,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
deleteComponentPlan "kubebb-system" "nginx-15.0.2-long-ready"
getHelmRevision "kubebb-system" "my-nginx-back" "1"
deleteComponentPlan "kubebb-system" "do-once-nginx-sample-15.0.2"

info "5.5 Verify can install to other namespace"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml --dry-run -o json | jq '.metadata.namespace="default"' | kubectl apply -f -
waitComponentPlanDone "default" "do-once-nginx-sample-15.0.2"
waitPodReady "default" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getHelmRevision "default" "my-nginx" "1"
deleteComponentPlan "default" "do-once-nginx-sample-15.0.2"

info "5.6 Verify can be successfully uninstalled when install failed"
kubectl apply -f config/samples/core_v1alpha1_componentplan_image_override.yaml --dry-run -o json | jq '.spec.wait=true' | jq '.spec.override.images[0].newTag="xxxxx"' | jq '.spec.timeoutSeconds=30' | kubectl apply -f -
getPodImage "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2" "docker.io/bitnami/nginx:xxxx"
waitComponentPlanRetryTime "kubebb-system" "nginx-15.0.2" "5"
deleteComponentPlan "kubebb-system" "nginx-15.0.2"

info "all finished! ✅"
