# Kubebb Core

[![codecov](https://codecov.io/gh/kubebb/core/branch/main/graph/badge.svg?token=TBPAVEZV2K)](https://codecov.io/gh/kubebb/core)

Kubebb Core provides core implementations on Component Lifecycle Management.Our design and development follows [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) which extends kubernetes APIs.

![arch](./assets/arch.png)

## Why Kubebb-Core?

- declaratively component lifecycle management which is fully compatible with Helm ecosystem
- combines with low-code platform to offer a full-stack solution for kubernetes application development and deployment
- automatically upgrade component with subscription
- flexible and powerful manifest override mechanism

## Documentation

To learn more about KubeBB Core,[go to complete documentation](https://kubebb.github.io/website/).

To get started quickly with KubeBB Core, [go to quick start](https://kubebb.github.io/website/docs/category/快速开始).

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

1. Enable events on `Component` changes
2. Adapt to Kubebb building base capabilities
3. Manage [building base](https://github.com/kubebb/building-base) as a component
4. Component Rating(Pre-Checks on Component) with the help of [Tekton](https://tekton.dev/)
5. Support ArgoCD in ComponentPlan

## Acknowledgement

This project is standing on the shoulders of giants. We would like to thank the following projects.

- [Helm](https://helm.sh/)
- [OLM](https://github.com/operator-framework/operator-lifecycle-manager)
- [Fluxcd](https://fluxcd.io/)
- [ArgoCD](https://argoproj.github.io/argo-cd/)

## Support

If you need support, start with the troubleshooting guide, or create GitHub [issues](https://github.com/kubebb/core/issues/new)
