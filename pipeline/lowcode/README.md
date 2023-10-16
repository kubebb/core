# Lowcode pipelines

> NOTE: All lowcode resources managed under namespace `yunti-system` by default

Here we use [tekton](https://tekton.dev/) to build and publish a lowcode component. The overall workflow is:

1. fetch source from minio
2. build dockerimage for lowcode component and push it to docker registry(dockerhub/private registry)
3. build helm charts for lowcode component with pre-defined [chart template](./chart-template/) and push this component to a private component repository(chartmuseum)


## Dependent components

- Component [Minio](https://github.com/kubebb/components/tree/main/charts/minio) to store lowcode component's source code
- Component [Tekton](https://github.com/kubebb/components/tree/main/charts/tekton-operator) to run the workflow

## Pipeline

We defined a [pipeline lowcode-publish](./pipeline.yaml) which contains 3 sequencial tasks:

- Task [fetch-source](./task-fetch-source.yaml)
- Task [build-image](./task-build-image.yaml)
- Task [build-chart](./task-build-chart.yaml)


### Parameters

|  Parameter |   Default |   Description     |
|---------------|----------------|----------------|
| SOURCE_MINIO_HOST | my-minio.kubebb-addons.svc.cluster.local | minio host/domain to fetch | 
| SOURCE_PATH | "" | The path where stores the component lowcode materials |
| SOURCE_SCHEMA_PATH | "schema.json" | The relative path of schema json file to `bucket/object/` |
| SOURCE_MINIO_ACCESS_KEY | "" |  |
| SOURCE_MINIO_SECRET_KEY | "" | |
| APP_IMAGE | "" |  The component's image name along with tag | 
| REPOSITORY_URL | http://chartmuseum.kubebb-addons.svc.cluster.local:8080 |  The url for the component repository |
| REPOSITORY_USER | "" |  The username for repository auth |
| REPOSITORY_PASSWORD | "" |  The password for the repository auth |

### Chart render

We use this [template](./chart-template/Chart.yaml) to render the chart.Below is the relevant mappings
|   Key in schema or others  |   Key in Chart  |   Description     |
|---------------|-----------------------------|----------------|
| Schema .version           | Chart.yaml version          |  Component portal entry and path |
| params IMAGE           | values.yaml image          |  Component portal entry and path |
| Schema .meta.namespace          | Chart.yaml name              |  Component name |
| Schema .meta.name               | Chart.yaml  annotations.core.kubebb.k8s.com.cn/displayname       |  Component display name |
| Schema .meta.description        | Chart.yaml  description           |  Component's description    |
| Schema .meta.git_url              | Chart.yaml  sources             |  Component's git url         |
| Schema .meta.basename           | templates.portal.yaml .spec.entry & .spec.path           |  Component portal entry and path |

## Sample

Clone this project:

```shell
git clone https://github.com/kubebb/core.git
cd core
```

### Prerequsites

0. deploy kubebb 

Follow [official document](https://kubebb.github.io/website/docs/quick-start/core_quickstart)

1. deploy minio 

```shell
    kubectl apply -f https://raw.githubusercontent.com/kubebb/components/main/examples/minio/componentplan.yaml
```

2. deploy chartmuseum 

```shell
    kubectl apply -f https://raw.githubusercontent.com/kubebb/components/main/examples/chartmuseum/componentplan.yaml
```

3. deploy tekton-operator with https://github.com/kubebb/components/tree/main/examples/tekton-operator

```shell
    kubectl apply -f https://raw.githubusercontent.com/kubebb/components/main/examples/tekton-operator/componentplan.yaml
```

4. deploy the lowcode pipelines

```shell
make deploy-lowcode
```

### Build component `hello-world`

#### 1. Upload component's schema to minio

- Bucket: `yunti`
- Object: `hello-world`

Upload `pipeline/lowcode/sample/schema.json` under `yunti/hello-world`


#### 2. Apply resources to pre-publish

1. Update dockerconfig

To push image to image registry,we mount this [`dockerconfig-secret`](./sample/pre-publish.yaml#1) in the kaniko pod. So make sure you have created this secret with the correct auth info.

2. Update Dockerfile

This pipelinerun use `dockerfile` configured in configmap [dockerfile-cm](./sample/pre-publish.yaml#13). Update according to your need.

3. Update pvc 

This pipelinerun requires a PVC `sample-publish-ws-pvc` to store source and artifacts. You need to update it based on your environment.

4. Apply this [pre-publish](./sample/pre-publish.yaml)

```shell
    kubectl apply -f pipeline/lowcode/sample/pre-publish.yaml
```

#### 4. Apply resources(pipelinerun) to build and publish this component

```shell
    kubectl apply -f pipeline/lowcode/sample/publish.yaml
```

#### 5. Watch pipelinerun

```shell
    kubectl get pods --watch -nyunti-system
```


#### 6. Get the component 

> Make sure you have add the above chartmuseum repo .In case you didnt, you can add it with this [componentplan](https://github.com/kubebb/components/blob/main/examples/chartmuseum/repository_chartmuseum.yaml)

```shell
❯ kubectl get components -nkubebb-system -l kubebb.component.repository=chartmuseum
NAME                          AGE
chartmuseum.hello-world   4m18s
```

#### 7. Deploy the above component

```shell
    kubectl apply -f pipeline/lowcode/sample/install-component.yaml
```

Check the install status:

```shell
kubectl get componentplan hello-world -oyaml
```

When `InstallSuccess`, you can access this component by:

1. do port-forward

```shell
 kubectl port-forward svc/hello-world 8066:8066
```

2. access in browser `http://localhost:8066/umi-demo-public`
