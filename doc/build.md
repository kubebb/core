初始化项目

```bash
bash hack/install-operator-sdk

operator-sdk init --domain kubebb.k8s.com.cn --component-config true --owner kubebb --project-name core --repo github.com/kubebb/core
```

创建一个 CRD

```bash
operator-sdk create api --resource --controller --namespaced=false --group core --version v1alpha1 --kind Repository
```

修复 CRD 的 api 或者 controller 后

```bash
make generate && make manifests
```

快速测试

```bash
# 创建一个测试集群
make kind
# 把CRD部署到这个集群中
make install
# 使用sample测试
kubectl apply -f config/sample/xxx
# 本地启用controller
make run
```

查看 makefile 帮助

```bash
make help
```
