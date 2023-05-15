# Kubebb Core

Kubebb Core provides core implementations on Component Lifecycle Management.Our design and development follows [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) which extends kubernetes APIs.

## Documentation

See our documentation on [Kubebb Site](http://kubebb.k8s.com.cn) for more details.

## Contribute to Kubebb-Core

If you want to contribute to Kubb Core, refer to [contribute guide](CONTRIBUTING.md).

## Roadmap

You can get what we're doing and plan to do here.

### v0.0.1

1. Component Repository

- Repository Server based on [chartmuseum](https://chartmuseum.com/docs/#)
- `Watcher` to watch on `Component` changes

2. Component Management

- CRUD on `Components` based on the `Watcher`
- Component Rating with the help of [Tekton](https://tekton.dev/)

3. Component Planning and Subscription

- Enable users subscribe on latest changes on `Component`
- Plan a component with the help of [Tekton](https://tekton.dev/)
- Customization on upgrade/downgrade solution on `Component`

## Support

If you need support, start with the troubleshooting guide, or create github [issues](https://github.com/kubebb/core/issues/new)
