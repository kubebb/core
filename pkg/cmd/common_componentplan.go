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

package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ghodss/yaml"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubebb/core/api/v1alpha1"
)

var (
	once sync.Once
	cc   client.Client
)

const (
	mustAddForClusterComponent = "ingress-nginx.controller.nodeSelector.kubernetes\\.io/hostname"
	clusterComponentName       = "cluster-component"
	installTemplate            = `apiVersion: core.kubebb.k8s.com.cn/v1alpha1
kind: ComponentPlan
metadata:
  name: cluster-component
  namespace: u4a-system
spec:
  approved: true
  name: cluster-component
  version: version
  component:
    name: kubebb.cluster-component
    namespace: kubebb-system`

	u4aValueYaml = `# Default values for u4a-system.
# This is a YAML-formatted file.
# Declare variables to be passed into your templates.

# You must check and update the value of each variable below
deploymentConfig:
  # bffServer info
  bffHost: portal.<replaced-ingress-nginx-ip>.nip.io
  clientName: bff-client
  clientId: bff-client
  clientSecret: 61324af0-1234-4f61-b110-ef57013267d6
  # BFF server info
  # The host Kubernetes cluster with OIDC enabled or use kube-oidc-proxy in front of k8s-apiserver
  # kube-oidc-proxy will be installed by default
  hostK8sApiWithOidc: https://k8s.<replaced-ingress-nginx-ip>.nip.io
  # Enable https for bff server
  bffHttpsEnabled: true

registryServer: docker.io

issuerConfig:
  # oidc certificate info
  # if using kube-odic-proxy, should add both oidc and proxy IP address here
  certificate:
    # MUST update this value
    oidcIPs:
      - <replaced-ingress-nginx-ip>
    # MUST update this value
    dnsNames:
      - portal.<replaced-ingress-nginx-ip>.nip.io
      - oidc-server
      - oidc-server.u4a-system
      - oidc-server.u4a-system.svc
      - kube-oidc-proxy
      - kube-oidc-proxy.u4a-system
      - kube-oidc-proxy.u4a-system.svc
      - k8s.<replaced-ingress-nginx-ip>.nip.io
  spec:
    # Use selfSigned or specified CA(such as CA from kubernetes)
    selfSigned: {}
    # ca:
    # secretName: k8s-ca-key-pair

# The ingress class id of the nginx ingress to expose the services for external access
ingress:
  name: portal-ingress

###############################################################################################
### Below is the configuration for each service, in most cases, you don't need to update them
### But update as you need if it's required, such as image, connector etc...
###############################################################################################
# Optional but the default: Use Kubernetes CRD for user provider - iam provider
iamProvider:
  enabled: true
  image: kubebb/iam-provider-ce:v0.1.0

# Required: Use dex as the odic service
oidcServer:
  enabled: true
  host: portal.<replaced-ingress-nginx-ip>.nip.io
  cert:
    ipAddresses: 
    - <replaced-ingress-nginx-ip>
    dnsNames:
      - portal.<replaced-ingress-nginx-ip>.nip.io
      - oidc-server
      - oidc-server.u4a-system
      - oidc-server.u4a-system.svc
      - kube-oidc-proxy
      - kube-oidc-proxy.u4a-system
      - kube-oidc-proxy.u4a-system.svc
      - k8s.<replaced-ingress-nginx-ip>.nip.io
  image: kubebb/oidc-server-ce:v0.1.0
  issuer: https://{{ .Values.deploymentConfig.bffHost }}/oidc
  storageType: kubernetes
  webHttps: 0.0.0.0:5556
  clientId: bff-client
  connectors:
    - type: k8scrd
      name: k8scrd
      id: k8scrd
      config:
        host: https://127.0.0.1:443
        insecureSkipVerify: true
  staticClients:
    - id: bff-client
      redirectURIs:
        - https://{{ .Values.deploymentConfig.bffHost }}/
      name: bff-client
      secret: 61324af0-1234-4f61-b110-ef57013267d6
  # Enable and update the ip if nip.io is NOT accessible in deployed environment
  hostConfig:
    enabled: true
    hostAliases:
      - hostnames:
          - portal.<replaced-ingress-nginx-ip>.nip.io
        ip: <replaced-ingress-nginx-ip>
  # only enable for debug purpose
  debug: false

# Optional but the default: BFF server for all API endpoints
bffServer:
  enabled: true
  image: kubebb/bff-server-ce:v0.1.4
  host: portal.<replaced-ingress-nginx-ip>.nip.io
  connectorId: k8scrd
  clientId: bff-client
  clientSecret: 61324af0-1234-4f61-b110-ef57013267d6
  # Enable and update the ip if nip.io is NOT accessible in deployed environment
  hostConfig:
    enabled: true
    hostAliases:
      - hostnames:
          - portal.<replaced-ingress-nginx-ip>.nip.io
        ip: <replaced-ingress-nginx-ip>

# Required: the host Kubernetes cluster with OIDC enabled
# or use kube-oidc-proxy in front of k8s-apiserver
k8s:
  hostK8sApiWithOidc: https://k8s.<replaced-ingress-nginx-ip>.nip.io  

# Generate tenant/namespace/user view for query
# Install if it's host cluster
resourceView:
  image: kubebb/resource-viewer-ce:v0.1.0

addon-component:
  enabled: true
  tenantManagement:
    image: kubebb/capsule-ce:v0.1.2-20221122
  kubeOidcProxy:
    image: kubebb/kube-oidc-proxy-ce:v0.3.0-20221008
    issuerUrl: https://portal.<replaced-ingress-nginx-ip>.nip.io/oidc
    clientId: bff-client
    ingress:
      enabled: true
      name: portal-ingress
      host: k8s.<replaced-ingress-nginx-ip>.nip.io
    certificate:
      ipAddresses:
        # MUST update this value to the host ip of kube-oidc-proxy
        - <replaced-ingress-nginx-ip>
      dnsNames:
        - kube-oidc-proxy
        - kube-oidc-proxy.u4a-system
        - kube-oidc-proxy.u4a-system.svc
    hostConfig:
      enabled: true
      hostAliases:
        - hostnames:
            # MUST update this value
            - portal.<replaced-ingress-nginx-ip>.nip.io
          # MUST update this value
          ip: <replaced-ingress-nginx-ip>

# Optional: Enable it if use LDAP as user provider
ldapProvider:
  enabled: false
  storageClass: openebs-hostpath
#  ldapImage: 172.22.96.19/u4a_system/openldap:1.5.0
#  ldapOrg: test
#  ldapDomain: test.com
#  ldapAdminPwd: xxx
#  ldapAdminImage: 172.22.96.19/u4a_system/phpldapadmin:stable

cluster-component:
  enabled: false`
)

func init() {
	Enroll(CLUSTERCOMPONENT, CommonInstaller)
	Enroll(U4A, CommonInstaller)
	Enroll(COMPONENTSTORE, CommonInstaller)
}

func initCli() {
	cfg, err := ctrl.GetConfig()
	if err != nil {
		panic(err)
	}

	fmt.Println("init controller runtime client")
	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	cc, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}
}

func CommonInstaller(cfg *Config) Installer {
	return &ClusterComponent{cfg: cfg}
}

type ClusterComponent struct {
	cfg *Config
}

func (c *ClusterComponent) Description() string {
	return fmt.Sprintf("Install %s", c.cfg.RegisterName)
}

func (c *ClusterComponent) Install(ctx context.Context) error {
	once.Do(initCli)
	componentPlan := v1alpha1.ComponentPlan{}
	if err := yaml.Unmarshal([]byte(installTemplate), &componentPlan); err != nil {
		return err
	}

	componentPlan.SetNamespace(c.cfg.Namespace)
	componentPlan.SetName(c.cfg.RegisterName)
	componentPlan.Spec.InstallVersion = c.cfg.Version
	componentPlan.Spec.Name = c.cfg.RegisterName
	componentPlan.Spec.ComponentRef.Namespace = c.cfg.Namespace
	componentPlan.Spec.Creator = "system:serviceaccount:u4a-system:kubebb-core"
	componentPlan.Spec.ComponentRef.Name = fmt.Sprintf("%s.%s", DEFAULTINSTALLREPO, c.cfg.RegisterName)

	if c.cfg.RegisterName == U4A {
		a := strings.ReplaceAll(u4aValueYaml, "<replaced-ingress-nginx-ip>", c.cfg.NodeIP)
		cm := corev1.ConfigMap{
			ObjectMeta: v1.ObjectMeta{
				Name:      "u4acm",
				Namespace: c.cfg.Namespace,
			},
			Data: map[string]string{
				"values.yaml": a,
			},
		}
		if err := cc.Create(ctx, &cm); err != nil {
			return err
		}
		componentPlan.Spec.Override.ValuesFrom = []*v1alpha1.ValuesReference{
			{
				Kind:      "ConfigMap",
				Name:      "u4acm",
				ValuesKey: "values.yaml",
			},
		}
	} else {
		componentPlan.Spec.Override.Set = c.cfg.Args.Values
	}
	err := cc.Create(ctx, &componentPlan)
	if c.cfg.RegisterName == CLUSTERCOMPONENT {
		// here need to wait for cert-manager to update the core to ensure that the webhook will work.
		return WaitDeployment(ctx, cc, c.cfg.Namespace, []string{"cert-manager", "cert-manager-cainjector", "cert-manager-webhook", "cluster-component-ingress-nginx-controller"}, 300)
	}
	return err
}

func (c *ClusterComponent) Upgrade(ctx context.Context) error {
	// not support
	return nil
}

func (c *ClusterComponent) Uninstall(ctx context.Context) {
	once.Do(initCli)
	componentPlan := v1alpha1.ComponentPlan{
		ObjectMeta: v1.ObjectMeta{
			Name:      c.cfg.RegisterName,
			Namespace: c.cfg.Namespace,
		},
	}
	_ = cc.Delete(ctx, &componentPlan)
}
