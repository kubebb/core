初始化项目
```bash
bash hack/install-operator-sdk

operator-sdk init --domain kubebb.k8s.com.cn --component-config true --owner kubebb --project-name core --repo github.com/kubebb/core
```

创建一个CRD
```bash
operator-sdk create api --resource --controller --namespaced=false --group core --version v1alpha1 --kind Repository
```

修复CRD的api或者controller后
```bash
make generate && make manifests
```
