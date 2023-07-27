## Usage

### Create RBAC

Task operation needs to create cm, so give its minimum permission: only allow to operate configmap

```shell
kubectl apply -f rbac/
```

### Create Pipeline

pipeline defines the tasks that need to be executed

```shell
kubectl apply -f pipeline.yaml
```

### Create Tasks

There are two tasks here:

- `helm-lint` is used to check whether the chart package of helm meets the requirements. 
- `rback` is used to generate the rabc permission map of the chart package.

```shell
kubectl apply -f tasks/
```

### Create PipelineRun

```shell
kubectl apply -f ./samples/pipeline-run.yaml

root@macbookpro:~/workspace/tekton-samples/core/dev# kubectl get cm|grep kubebb
kubebb.kubebb.v0.0.1   1      7m11s
```

