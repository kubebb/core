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

KindName="kubebb-core"
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
		versions=$(kubectl -n${namespace} get components.core.kubebb.k8s.com.cn ${componentName} --ignore-not-found=true -ojson | jq -r '.status.versions|length')
		if [[ $versions -ne 0 ]]; then
			echo "component ${componentName} already have version information and can be installed"
			break
		fi
		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			kubectl -n${namespace} get components
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
		doneConds=$(kubectl -n${namespace} get ComponentPlan ${componentPlanName} --ignore-not-found=true -ojson | jq -r '.status.conditions' | jq 'map(select(.type == "Succeeded"))|map(select(.status == "True"))|length')
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
	helmReleaseShouldDelete=$3
	START_TIME=$(date +%s)
	helmReleaseName=$(kubectl get ComponentPlan -n${namespace} ${componentPlanName} --ignore-not-found=true -ojson | jq -r '.spec.name')
	if [[ $helmReleaseName == "" ]]; then
		echo "componentPlan ${componentPlanName} has no release name"
		kubectl get ComponentPlan -n${namespace} ${componentPlanName} -oyaml
		exit 1
	fi
	while true; do
		kubectl -n${namespace} delete ComponentPlan ${componentPlanName} --wait --ignore-not-found=true
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
		results=$(helm status -n ${namespace} ${helmReleaseName})
		if [[ $helmReleaseShouldDelete == "true" ]] && [[ $results == "" ]]; then
			echo "helm release should remove, and helm status also show componentplan:${componentPlanName} 's release:${helmReleaseName} removed"
			break
		elif [[ $helmReleaseShouldDelete == "false" ]] && [[ $results != "" ]]; then
			echo "helm release should not remove, and helm status also show componentplan:${componentPlanName} 's release:${helmReleaseName} not removed"
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
	START_TIME=$(date +%s)
	while true; do
		latestValue=$(kubectl -n${namespace} get ComponentPlan ${componentPlanName} --ignore-not-found=true -ojson | jq -r '.status.latest')
		if [[ $latestValue == $want ]]; then
			echo "componentPlan ${componentPlanName} status.latest is $latestValue"
			break
		fi

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			echo "Timeout reached. componentPlan ${componentPlanName} status.latest is $latestValue, not $want"
			kubectl -n${namespace} get ComponentPlan ${componentPlanName} -o yaml
			exit 1
		fi
		sleep 5
	done
}

function waitComponentPlanRetryTime() {
	namespace=$1
	componentPlanName=$2
	retryTimeWant=$3
	START_TIME=$(date +%s)
	while true; do
		anno=$(kubectl -n${namespace} get ComponentPlan ${componentPlanName} --ignore-not-found=true -ojson | jq -r '.metadata.annotations["core.kubebb.k8s.com.cn/componentplan-retry"]')
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

function checkPortalStatus() {
	portalName=$1
	expectConflictsInEntry=$2
	expectConflictsInPath=$3

	START_TIME=$(date +%s)
	while true; do
		conflictsInEntry=$(kubectl get Portal ${portalName} -ojson | jq -r 'if (.status.conflictsInEntry| type) == "array" then .status.conflictsInEntry| sort | join(",") else empty end')
		conflictsInPath=$(kubectl get Portal ${portalName} -ojson | jq -r 'if (.status.conflictsInPath| type) == "array" then .status.conflictsInPath| sort | join(",") else empty end')

		if [[ $expectConflictsInEntry == $conflictsInEntry ]] && [[ $expectConflictsInPath == $conflictsInPath ]]; then
			info "Portal ${portalName} has the correct status in conflicted entry and path"
			break
		fi

		info "expectConflictsInEntry:$expectConflictsInEntry  Actual:$conflictsInEntry"
		info "expectConflictsInPath:$expectConflictsInPath  Actual:$conflictsInPath"

		CURRENT_TIME=$(date +%s)
		ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
		if [ $ELAPSED_TIME -gt $TimeoutSeconds ]; then
			error "Timeout reached"
			kubectl get Portal -o yaml
			exit 1
		fi
		sleep 5
	done
}

info "1. create kind cluster"
make kind

info "2. install kubebb core"
info "2.1 install cert-manager for kubebb core webhook"
helm repo add --force-update jetstack https://charts.jetstack.io
helm repo update jetstack
helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace \
	--version v1.12.0 \
	--set prometheus.enabled=false \
	--set installCRDs=true

info "2.2 deploy kubebb/core"
docker tag kubebb/core:latest kubebb/core:example-e2e
kind load docker-image kubebb/core:example-e2e --name=$KindName
make deploy IMG="kubebb/core:example-e2e"
kubectl wait deploy -n kubebb-system kubebb-controller-manager --for condition=Available=True

info "3 try to verify that the common steps are valid"
info "3.1 create bitnami repository"
kubectl apply -f config/samples/core_v1alpha1_repository_bitnami.yaml
waitComponentStatus "kubebb-system" "repository-bitnami-sample.nginx"

info "3.2 create nginx componentplan"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml
waitComponentPlanDone "kubebb-system" "do-once-nginx-sample-15.0.2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getHelmRevision "kubebb-system" "my-nginx" "1"
deleteComponentPlan "kubebb-system" "do-once-nginx-sample-15.0.2" "true"

info "3.3 create nginx-15.0.2 componentplan to verify imageOverride in componentPlan is valid"
kubectl apply -f config/samples/core_v1alpha1_componentplan_image_override.yaml
waitComponentPlanDone "kubebb-system" "nginx-15.0.2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getPodImage "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2" "docker.io/bitnami/nginx:latest"
getHelmRevision "kubebb-system" "my-nginx" "1"
deleteComponentPlan "kubebb-system" "nginx-15.0.2" "true"

info "3.4 create nginx-replicas-example-1/2 componentplan to verify value override in componentPlan is valid"
kubectl apply -f config/samples/core_v1alpha1_nginx_replicas2_componentplan.yaml
waitComponentPlanDone "kubebb-system" "nginx-replicas-example-1"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx-replicas-example-1,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getDeployReplicas "kubebb-system" "my-nginx-replicas-example-1" "2"
getHelmRevision "kubebb-system" "my-nginx-replicas-example-1" "1"
deleteComponentPlan "kubebb-system" "nginx-replicas-example-1" "true"

waitComponentPlanDone "kubebb-system" "nginx-replicas-example-2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx-replicas-example-2,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getDeployReplicas "kubebb-system" "my-nginx-replicas-example-2" "2"
getHelmRevision "kubebb-system" "my-nginx-replicas-example-2" "1"
deleteComponentPlan "kubebb-system" "nginx-replicas-example-2" "true"

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

info "5.4 rollback nginx to older version"
kubectl patch componentplan -n kubebb-system do-once-nginx-sample-15.0.2 --type=json \
	-p='[{"op": "add", "path": "/metadata/labels/core.kubebb.k8s.com.cn~1rollback", "value": "true"}]'
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getHelmRevision "kubebb-system" "my-nginx" "4"
validateComponentPlanStatusLatestValue "kubebb-system" "do-once-nginx-sample-15.1.0" "false"
validateComponentPlanStatusLatestValue "kubebb-system" "do-once-nginx-sample-15.0.2" "true"
deleteComponentPlan "kubebb-system" "do-once-nginx-sample-15.1.0" "false"
deleteComponentPlan "kubebb-system" "do-once-nginx-sample-15.0.2" "true"

info "5.5 Verify long running componentPlan install don not block others to install"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan_long_ready.yaml
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml --dry-run=client -o json | jq '.spec.name="my-nginx-back"' | kubectl apply -f -
waitComponentPlanDone "kubebb-system" "do-once-nginx-sample-15.0.2"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-nginx-back,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
deleteComponentPlan "kubebb-system" "nginx-15.0.2-long-ready" "false"
getHelmRevision "kubebb-system" "my-nginx-back" "1"
deleteComponentPlan "kubebb-system" "do-once-nginx-sample-15.0.2" "true"

info "5.6 Verify can install to other namespace"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml --dry-run=client -o json | jq '.metadata.namespace="default"' | kubectl apply -f -
waitComponentPlanDone "default" "do-once-nginx-sample-15.0.2"
waitPodReady "default" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2"
getHelmRevision "default" "my-nginx" "1"
deleteComponentPlan "default" "do-once-nginx-sample-15.0.2" "true"

info "5.7 Verify can be successfully uninstalled when install failed"
kubectl apply -f config/samples/core_v1alpha1_componentplan_image_override.yaml --dry-run=client -o json | jq '.spec.wait=true' | jq '.spec.override.images[0].newTag="xxxxx"' | jq '.spec.timeoutSeconds=30' | kubectl apply -f -
getPodImage "kubebb-system" "app.kubernetes.io/instance=my-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.0.2" "docker.io/bitnami/nginx:xxxx"
waitComponentPlanRetryTime "kubebb-system" "nginx-15.0.2" "5"
deleteComponentPlan "kubebb-system" "nginx-15.0.2" "true"

info "5.8 verify common user can create componentplan, but they must have permissions."
info "5.8.1 create a sa with deploy and svc permissions, but no ingress"
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  creationTimestamp: null
  name: usera
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: usera
  namespace: default
rules:
- apiGroups: ["", "extensions", "apps", "core.kubebb.k8s.com.cn"]
  resources: ["*"]
  verbs: ["*"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: usera
  namespace: default
subjects:
- kind: User
  name: usera
  apiGroup: rbac.authorization.k8s.io
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: usera
EOF
info "5.8.2 Verify that this user can't create ingress"
kubectl create ingress simple --rule="foo.com/bar=svc1:8080" --as=usera || true
info "5.8.3 Use this user to create componentplan"
kubectl apply -f config/samples/core_v1alpha1_nginx_componentplan.yaml --dry-run=client -o json | jq '.metadata.namespace="default"' | jq '.spec.override.set[0]="ingress.enabled=true"' | kubectl apply --as=usera -f -
waitComponentPlanRetryTime "default" "do-once-nginx-sample-15.0.2" "5"
info "5.8.4 verify this componentplan will failed, show error log"
kubectl get cpl do-once-nginx-sample-15.0.2 '--output=jsonpath={.status.conditions[?(@.type=="Actioned")]}{"\n"}'

info "6 Verify that helm repo with basic auth"
info "6.1 Verify that helm repo add with basic auth"
kubectl apply -f config/samples/core_v1alpha1_repository_kubebb.yaml
waitComponentStatus "kubebb-system" "repository-kubebb.chartmuseum"
info "6.1.1 Plan a private repository with chartmuseum"
kubectl apply -f config/samples/core_v1alpha1_componentplan_chartmuseum.yaml
waitPodReady "kubebb-system" "app.kubernetes.io/instance=my-chartmuseum"
info "6.1.2 Verify private chartmuseum service is well running"
export POD_NAME=$(kubectl get pods --namespace kubebb-system -l app.kubernetes.io/instance=my-chartmuseum -o jsonpath="{.items[0].metadata.name}")
nohup kubectl port-forward $POD_NAME 8088:8080 --namespace kubebb-system >/dev/null 2>&1 &
curl --silent --retry 3 --retry-delay 5 --retry-connrefused -u admin:password http://localhost:8088

info "6.2 Verify that helm install with basic auth"
info "6.2.1 Push a chart to private chartmuseum"
curl --silent --retry 3 --retry-delay 5 -O https://charts.bitnami.com/bitnami/nginx-15.1.2.tgz
curl --silent --retry 3 --retry-delay 5 -u admin:password --data-binary "@nginx-15.1.2.tgz" http://localhost:8088/api/charts
echo "\n"
info "6.2.2 Add this private chartmuseum repository(basic auth enabled) to kubebb"
kubectl apply -f config/samples/core_v1alpha1_repository_chartmuseum.yaml
waitComponentStatus "kubebb-system" "repository-chartmuseum.nginx"
info "6.2.3 Plan a nignx with private chartmuseum(basic auth enabled) "
kubectl apply -f config/samples/core_v1alpha1_componentplan_mynginx.yaml
waitComponentPlanDone "kubebb-system" "mynginx"

info "7 try to verify that the common steps are valid to oci types"
info "7.1 create oci repository"
kubectl apply -f config/samples/core_v1alpha1_repository_oci.yaml
waitComponentStatus "kubebb-system" "repository-oci-sample.nginx"
oci_digest=$(kubectl -n${namespace} get components ${componentName} -ojson | jq -r '.status.versions[0].digest')
fixed_nginx_digest="d9459e1206a4f5a8e0d7c5da8a306ab9b1ba5d7182ae671610b5699250ea45f8"
echo "digest: ${oci_digest}"
if [[ ${oci_digest} != ${fixed_nginx_digest} ]]; then
	echo "digest has wrong value"
	exit 1
fi

info "7.2 create oci componentplan"
kubectl apply -f config/samples/core_v1alpha1_oci_componentplan.yaml
waitComponentPlanDone "kubebb-system" "do-once-oci-sample-15.1.0"
waitPodReady "kubebb-system" "app.kubernetes.io/instance=oci-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.1.0"
getHelmRevision "kubebb-system" "oci-nginx" "1"

info "7.3 create new componentPlan to update oci release replicas to 2, to valid upgrade"
kubectl apply -f config/samples/core_v1alpha1_oci_componentplan.yaml --dry-run=client -o json | jq '.metadata.name="do-once-oci-sample-15.1.0-2"' | jq '.spec.override.values.replicaCount=2' | kubectl apply -f -
waitPodReady "kubebb-system" "app.kubernetes.io/instance=oci-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.1.0"
getHelmRevision "kubebb-system" "oci-nginx" "2"
getDeployReplicas "kubebb-system" "oci-nginx" "2"
waitComponentPlanDone "kubebb-system" "do-once-oci-sample-15.1.0-2"
validateComponentPlanStatusLatestValue "kubebb-system" "do-once-oci-sample-15.1.0-2" "true"
validateComponentPlanStatusLatestValue "kubebb-system" "do-once-oci-sample-15.1.0" "false"

info "7.4 rollback nginx to older version, to valid rollback"
kubectl patch componentplan -n kubebb-system do-once-oci-sample-15.1.0 --type=json \
	-p='[{"op": "add", "path": "/metadata/labels/core.kubebb.k8s.com.cn~1rollback", "value": "true"}]'
waitPodReady "kubebb-system" "app.kubernetes.io/instance=oci-nginx,app.kubernetes.io/managed-by=Helm,helm.sh/chart=nginx-15.1.0"
getHelmRevision "kubebb-system" "oci-nginx" "3"
validateComponentPlanStatusLatestValue "kubebb-system" "do-once-oci-sample-15.1.0-2" "false"
validateComponentPlanStatusLatestValue "kubebb-system" "do-once-oci-sample-15.1.0" "true"

info "7.5 remove them, to valid uninstall"
deleteComponentPlan "kubebb-system" "do-once-oci-sample-15.1.0-2" "false"
deleteComponentPlan "kubebb-system" "do-once-oci-sample-15.1.0" "true"

info "8 try to verify portal works correctly"
info "8.1 create portal-example which has no conflicts"
kubectl apply -f config/samples/core_v1alpha1_portal.yaml
checkPortalStatus "portal-example" "" ""

info "8.2 create portal-example-another which contains conflicts in entry"
kubectl apply -f config/samples/core_v1alpha1_portal_another.yaml
checkPortalStatus "portal-example" "portal-example-another" ""
checkPortalStatus "portal-example-another" "portal-example" ""

info "8.3 create a duplicate portal which both conflicts in entry and path"
kubectl apply -f config/samples/core_v1alpha1_portal_duplicate.yaml
checkPortalStatus "portal-example" "portal-example-another,portal-example-duplicate" "portal-example-duplicate"
checkPortalStatus "portal-example-another" "portal-example,portal-example-duplicate" ""
checkPortalStatus "portal-example-duplicate" "portal-example,portal-example-another" "portal-example"

info "8.4 update portal-exmaple-another to have confilicts both in entry and path"
kubectl apply -f config/samples/core_v1alpha1_portal_another_change.yaml
checkPortalStatus "portal-example" "portal-example-another,portal-example-duplicate" "portal-example-another,portal-example-duplicate"
checkPortalStatus "portal-example-another" "portal-example,portal-example-duplicate" "portal-example,portal-example-duplicate"
checkPortalStatus "portal-example-duplicate" "portal-example,portal-example-another" "portal-example,portal-example-another"

info "8.5 delete the portal-example-another portal and check the portal conflicts"
kubectl delete Portal portal-example-another
checkPortalStatus "portal-example" "portal-example-duplicate" "portal-example-duplicate"
checkPortalStatus "portal-example-duplicate" "portal-example" "portal-example"

info "9 try to verify that the common steps are valid to subscription"
info "9.1 create loki subscription with schedule to run after 5 minutes"
current_time_seconds=$(date +%s)
current_time=$(date +"%T")
new_mins_seconds=$((current_time_seconds + 300))
current_min=$(date +%-M)
new_min=$((current_min + 5))
if [ $new_min -gt 59 ]; then
	new_min=$((new_min - 60))
fi
new_mins_after_cron="$new_min * * * *"
echo "Current time: $current_time, 5 minutes after crontab: $new_mins_after_cron"
kubectl apply -f config/samples/core_v1alpha1_loki_subscription_schedule.yaml --dry-run=client -o json | jq --arg new_mins_after_cron "$new_mins_after_cron" '.spec.schedule=$new_mins_after_cron' | kubectl apply -f -
while true; do
	componentplanCreateTime=$(kubectl -nkubebb-system get ComponentPlan -l core.kubebb.k8s.com.cn/subscription-name=loki-sample-schedule -o jsonpath="{.items[0].metadata.creationTimestamp}" --ignore-not-found)
	if [[ -n $componentplanCreateTime ]]; then
		echo "componentplanCreateTime: $componentplanCreateTime"
		date=${componentplanCreateTime:0:10}
		time=${componentplanCreateTime:11:8}
		timestamp_seconds=$(date -d "$date $time" +%s)
		diff=$((timestamp_seconds - new_mins_seconds))
		if [ $diff -lt 0 ]; then
			diff=$((diff * -1))
		fi
		if [ $diff -lt 120 ]; then
			break
		else
			error "diff is too long :$diff"
			exit 1
		fi
		break
	fi

	CURRENT_TIME=$(date +%s)
	ELAPSED_TIME=$((CURRENT_TIME - START_TIME))
	if [ $ELAPSED_TIME -gt 1800 ]; then
		error "Timeout reached"
		kubectl get ComponentPlan -A -oyaml
		kubectl get Subscription -A -o yaml
		exit 1
	fi
	sleep 5
done

info "all finished! ✅"
