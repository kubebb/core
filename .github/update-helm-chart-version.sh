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
REPO_NAME=kubebb-components
REPO=$GITHUB_USERNAME/$REPO_NAME
BRANCH=auto-sync
gh auth setup-git
git config --global user.email "$GITHUB_USERNAME@users.noreply.github.com"
git config --global user.name "$GITHUB_USERNAME"
function updateChartVersion() {
	local RE='[^0-9]*\([0-9]*\)[.]\([0-9]*\)[.]\([0-9]*\)\([0-9A-Za-z-]*\)'
	eval MAJOR=$(echo $1 | sed -e "s#$RE#\1#")
	eval MINOR=$(echo $1 | sed -e "s#$RE#\2#")
	eval PATCH=$(echo $1 | sed -e "s#$RE#\3#")
	eval SPECIAL=$(echo $1 | sed -e "s#$RE#\4#")
	PATCH=$((PATCH + 1))
	if [[ $SPECIAL == "" ]]; then
		eval $2=v$MAJOR.$MINOR.$PATCH
	else
		eval $2=v$MAJOR.$MINOR.$PATCH.$SPECIAL
	fi
}

echo "1.get kubebb-core latest version"
latestTag=$(git describe --abbrev=0 --tags)
echo "kubebb-core latest tag is $latestTag"

echo "2.get kubebb-core chart version"
cd /tmp/
gh repo clone $REPO
cd $REPO_NAME
git checkout -b $BRANCH
git fetch upstream
git reset --hard upstream/main
imageVersion=$(grep 'image: kubebb/core:' charts/kubebb-core/values.yaml | cut -d ':' -f3)
inChartVersion=$(grep 'version:' charts/kubebb-core/Chart.yaml | cut -d ' ' -f2)
appVersion=$(grep 'appVersion:' charts/kubebb-core/Chart.yaml | cut -d ' ' -f2)

echo "3. update chart image version in values.yaml"
if [[ $latestTag == $imageVersion ]]; then
	echo "same image version in values.yaml, do not update it"
else
	sed -i "s/\(image:\) .*/\1 kubebb\/core:${latestTag}/" charts/kubebb-core/values.yaml
	git add charts/kubebb-core/values.yaml
fi

echo "3. update chart app version in Chart.yaml"
if [[ $latestTag == $appVersion ]]; then
	echo "same app version in Chart.yaml, do not update it"
else
	sed -i "s/\(appVersion:\) .*/\1 ${latestTag}/" charts/kubebb-core/Chart.yaml
	git add charts/kubebb-core/Chart.yaml
fi

echo "4. update chart crds"
cp -r ${GITHUB_WORKSPACE}/config/crd/bases/* charts/kubebb-core/crds/
git add charts/kubebb-core/crds

echo "5. if the above content changes, update the chart version"
if [[ -n $(git status --porcelain) ]]; then
	newChartVersion=0
	updateChartVersion $inChartVersion newChartVersion
	sed -i "s/\(version:\) .*/\1 ${newChartVersion}/" charts/kubebb-core/Chart.yaml
	git add charts/kubebb-core/Chart.yaml
else
	echo "no file chaged, just exit"
	exit 0
fi

echo "6. sync pipelines"
cp -r ${GITHUB_WORKSPACE}/pipeline/* charts/kubebb-core/files/
git add charts/kubebb-core/files

echo "7. git push and create pull request"
git commit -m "🤖 auto sync kubebb-core release:$latestTag"
git push --force origin $BRANCH
gh repo set-default kubebb/components
# must add --head arg, see https://github.com/cli/cli/issues/6485
gh pr create --title "🤖 Auto Sync Kubebb Core release $latestTag" --body "auto sync" --head $GITHUB_USERNAME:$BRANCH --base main

echo "all finished! ✅"
