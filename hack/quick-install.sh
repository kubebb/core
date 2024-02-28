#!/bin/bash

echo "> input nodename: "
read nodename

echo "> input reponame(used in helm repo add): "
read reponame

if [[ ${#nodename} -eq 0 ]]; then
	nodename="kubebb"
fi

if [[ ${#reponame} -eq 0 ]]; then
	reponame="kubebb"
fi

nodeip=$(kubectl get node $nodename --no-headers -owide | awk '{print $6}')
echo -e "nodename: $nodename, nodeip: $nodeip\n"

if [[ ${#nodeip} -eq 0 ]]; then
	echo -e "\033[31mUnable to find node ip. enter correct node name\033[0m"
	exit 1
fi

echo "1. add repo as ${reponame}"

helm repo add $reponame https://kubebb.github.io/components 2>/dev/null
helm repo update $reponame 2>/dev/null

echo -e "\n2. create namespace u4a-system, kubebb-system"
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: u4a-system
EOF

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: kubebb-system
EOF

echo -e "\n3. install kubebb-core in namesapce kubebb-system with name kubebb-core"

errmsg=$(helm -nkubebb-system install kubebb-core ${reponame}/kubebb-core --wait 2>&1 >/dev/null)
if [[ $? -ne 0 ]]; then
	echo -e "\033[31m[KUBEBB-CORE-COMPNENT]: ${errmsg}\033[0m"
	exit 1
else
	echo -e "\033[32mkubebb-core-component installed successfully\033[0m"
fi

echo -e "\n4. install the cluster-component in namespace u4a-system with name cluster-component"

if [[ -d "cluster-component" ]]; then
	rm -rf cluster-component
fi

errmsg=$(helm pull --untar=true ${reponame}/cluster-component 2>&1 >/dev/null)
if [[ $? -ne 0 ]]; then
	echo -e "\033[31m[CLUSTER-COMPNENT]: ${errmsg}\033[0m"
	exit 1
fi

echo "4.1 replace <replaced-ingress-node-name> in values.yaml"
sed -e "s/<replaced-ingress-node-name>/${nodename}/g" cluster-component/values.yaml >cluster-component/values-tmp.yaml

echo "4.2 intall cluster-component"
errmsg=$(helm -nu4a-system install cluster-component ${reponame}/cluster-component -f cluster-component/values-tmp.yaml --wait 2>&1 >/dev/null)

if [[ $? -ne 0 ]]; then
	echo -e "\033[31m[CLUSTER-COMPNENT]: ${errmsg}\033[0m"
	exit 1
else
	echo -e "\033[32mcluster-component installed successfully\033[0m"
fi

echo -e "\n5. install the u4a-component in namespace u4a-system with name u4a-component"

if [[ -d "u4a-component" ]]; then
	rm -rf u4a-component
fi

errmsg=$(helm pull --untar=true ${reponame}/u4a-component 2>&1 >/dev/null)
if [[ $? -ne 0 ]]; then
	echo -e "\033[31m[U4A-COMPNENT]: ${errmsg}\033[0m"
	exit 1
fi

echo "5.1 replace <replaced-ingress-nginx-ip> in values.yaml"
sed -e "s/<replaced-ingress-nginx-ip>/${nodeip}/g" u4a-component/values.yaml >u4a-component/values-tmp.yaml

echo "5.2 install u4a-compnent"
errmsg=$(helm -nu4a-system install u4a-component ${reponame}/u4a-component -f u4a-component/values-tmp.yaml --wait 2>&1 >/dev/null)
if [[ $? -ne 0 ]]; then
	echo -e "\033[31m[U4A-COMPNENT]: ${errmsg}\033[0m"
	exit 1
else
	echo -e "\033[32mu4a-component installed successfully\033[0m"
fi

echo -e "\n\033[32mDone\033[0m"
