# Kubebb Core

[![codecov](https://codecov.io/gh/kubebb/core/branch/main/graph/badge.svg?token=TBPAVEZV2K)](https://codecov.io/gh/kubebb/core)

Kubebb Core provides core implementations on Component Lifecycle Management.Our design and development follows [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) which extends kubernetes APIs.

![arch](./assets/arch.png)

## Why Kubebb-Core?

TODO...

## Documentation

See our documentation on [Kubebb Site](http://kubebb.k8s.com.cn) for more details.

## Contribute to Kubebb-Core

If you want to contribute to Kubb Core, refer to [contribute guide](CONTRIBUTING.md).

## Roadmap

You can get what we're doing and plan to do here.

### v0.1.0

1. Component Repository Management

- Support `Repository Server` which is compatible with Helm Repository
- `Watcher` to watch on `Component` changes

2. Component Management

- CRUD on `Components` by the `Watcher`

3. ComponentPlan and Subscription

- Enable users subscribe on latest changes on `Component`
- Plan a component deployment with `ComponentPlan`

### v0.2.0

1. Component Rating(Pre-Checks on Component) with the help of [Tekton](https://tekton.dev/)
2. Enable events on `Component` changes
3. Adapt to Kubebb building base capabilities
4. Support ArgoCD in ComponentPlan

## Acknowledgement

This project is standing on the shoulders of giants. We would like to thank the following projects.

- [Helm](https://helm.sh/)
- [OLM](https://github.com/operator-framework/operator-lifecycle-manager)
- [Fluxcd](https://fluxcd.io/)
- [ArgoCD](https://argoproj.github.io/argo-cd/)

## Support

If you need support, start with the troubleshooting guide, or create github [issues](https://github.com/kubebb/core/issues/new)
