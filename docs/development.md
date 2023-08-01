# Development

## Init a new operator project

```bash
bash hack/install-operator-sdk

operator-sdk init --domain kubebb.k8s.com.cn --component-config true --owner kubebb --project-name core --repo github.com/kubebb/core
```

## Create a CRD

```bash
operator-sdk create api --resource --controller --namespaced=false --group core --version v1alpha1 --kind Repository
```

## Create a WebHook

```bash
ooperator-sdk create webhook --group core --version v1alpha1 --kind ComponentPlan --defaulting --programmatic-validation --verbose
```
And change to `CustomDefaulter` and `CustomValidator` in `webhook.go` file.

### Regenerate after changes on CRD

```bash
make generate && make manifests
```

## Test locally

### Prepare a kubernetes cluster

1. create a kind cluster

```bash
make kind
```

2. install kubebb crds

```bash
make install
```

### Run kubebb core controller

#### Run locally

```bash
make run
```

#### Run in kind cluster

1. build docker image

```bash
make docker-build
```

2. load docker image to kind cluster

```bash
make kind-load
```

3. deploy controller in kind cluster

```bash
make deploy
```

## Help for makefile

```bash
❯ make help

Usage:
  make <target>

General
  help             Display this help.

Development
  manifests        Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
  generate         Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
  fmt              Run go fmt against code.
  vet              Run go vet against code.
  test             Run tests.

Build
  build            Build manager binary.
  docker-build     Build docker image with the manager.
  docker-push      Push docker image with the manager.

Deployment
  install          Install CRDs into the K8s cluster specified in ~/.kube/config.
  uninstall        Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
  run              Run a controller from your host.
  deploy           Deploy controller to the K8s cluster specified in ~/.kube/config.
  undeploy         Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
  kind             Install a kind cluster.
  unkind           Uninstall a kind cluster.
  chartmuseum      Install a chartmuseum in kubebb-system.
  unchartmuseum    Uninstall a chartmuseum from kubebb-system.

Build Dependencies
  kustomize        Download kustomize locally if necessary.
  controller-gen   Download controller-gen locally if necessary.
  envtest          Download envtest-setup locally if necessary.
  bundle           Generate bundle manifests and metadata, then validate generated files.
  bundle-build     Build the bundle image.
  bundle-push      Push the bundle image.
  opm              Download opm locally if necessary.
  catalog-build    Build a catalog image.
  catalog-push     Push a catalog image.
```
