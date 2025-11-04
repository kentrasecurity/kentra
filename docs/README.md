# KTtack Documentation Index

Welcome to the KTtack documentation. This directory contains comprehensive guides for installing, using, and understanding the KTtack Kubernetes Operator.

## Quick Navigation

### For First-Time Users
Start here if you're new to KTtack:

1. **[Getting Started](./GETTING_STARTED.md)** (5 minutes)
   - Create your first security test
   - Monitor execution
   - View results

2. **[Installation Guide](./INSTALLATION_GUIDE.md)** (15 minutes)
   - Multiple installation methods
   - Post-installation configuration
   - Troubleshooting

### For Developers and Architects
Understand how KTtack works:

1. **[Architecture Guide](./ARCHITECTURE.md)**
   - System design overview
   - Component interaction
   - Reconciliation flow
   - Job and CronJob management

2. **[API Reference](./API_REFERENCE.md)**
   - Complete CRD specification
   - Field reference
   - Validation rules
   - Code examples

### For Operations Teams
Configure and operate KTtack:

1. **[Fluent Bit Sidecar Documentation](./FLUENT_BIT_SIDECAR.md)**
   - Centralized logging setup
   - Loki integration
   - Log forwarding configuration

## Documentation by Topic

### Installation & Setup
- [Installation Guide](./INSTALLATION_GUIDE.md) - Multiple installation methods and verification
- [Getting Started](./GETTING_STARTED.md) - Quick start after installation

### Usage & Examples
- [Getting Started](./GETTING_STARTED.md) - First security test examples
- [API Reference](./API_REFERENCE.md) - Complete resource specifications and examples

### Architecture & Design
- [Architecture Guide](./ARCHITECTURE.md) - System design and components
- [API Reference](./API_REFERENCE.md) - API design and CRDs

### Operations & Monitoring
- [Fluent Bit Sidecar Documentation](./FLUENT_BIT_SIDECAR.md) - Logging configuration
- [Installation Guide](./INSTALLATION_GUIDE.md) - Post-installation configuration

## Installation Methods Comparison

| Method | Best For | Complexity | Speed |
|--------|----------|-----------|-------|
| [Pre-built Image](./INSTALLATION_GUIDE.md#method-1-from-pre-built-release) | Production deployments | Low | Fast |
| [Build from Source](./INSTALLATION_GUIDE.md#method-2-build-from-source) | Development environments | Medium | Medium |
| [Kustomize Bundle](./INSTALLATION_GUIDE.md#method-3-using-kustomize-bundle-distribution) | Distribution to users | Low | Fast |
| [Helm Chart](./INSTALLATION_GUIDE.md#method-4-using-helm-chart-optional) | Advanced customization | Medium | Medium |

## Common Tasks

### Get Started
1. Read [Installation Guide](./INSTALLATION_GUIDE.md)
2. Deploy KTtack to your cluster
3. Follow [Getting Started](./GETTING_STARTED.md)
4. Create your first SecurityAttack resource

### Understand the System
1. Review [Architecture Guide](./ARCHITECTURE.md) overview
2. Read about [Reconciliation Flow](./ARCHITECTURE.md#controller-reconciliation-flow)
3. Study [Job Management](./ARCHITECTURE.md#job-and-cronjob-management)

### Configure Advanced Features
1. Set up [Fluent Bit Logging](./FLUENT_BIT_SIDECAR.md)
2. Configure tool specifications
3. Add custom monitoring

### Troubleshoot Issues
1. Check [Installation Guide](./INSTALLATION_GUIDE.md#troubleshooting-installation) troubleshooting section
2. Review [Getting Started](./GETTING_STARTED.md#troubleshooting) troubleshooting section
3. Check KTtack controller logs

## Key Concepts

### SecurityAttack Resource
A Kubernetes resource defining a one-time or periodic security testing operation.

**Learn more:** [API Reference](./API_REFERENCE.md#securityattack-api) | [Getting Started Examples](./GETTING_STARTED.md)

### Custom Resource Definitions (CRDs)
KTtack extends Kubernetes with `SecurityAttack`, `Enumeration`, and `Liveness` resource types.

**Learn more:** [Architecture - CRDs](./ARCHITECTURE.md#custom-resource-definitions-crds) | [API Reference](./API_REFERENCE.md)

### Controller Reconciliation
KTtack uses Kubernetes operators pattern to watch resources and create corresponding Jobs.

**Learn more:** [Architecture - Reconciliation Flow](./ARCHITECTURE.md#controller-reconciliation-flow)

### Fluent Bit Sidecar
Automatic log collection and forwarding to Loki for centralized log aggregation.

**Learn more:** [Fluent Bit Documentation](./FLUENT_BIT_SIDECAR.md)

## Cron Schedule Reference

Common cron schedule examples for periodic security testing:

| Schedule | Description | Example Use Case |
|----------|-------------|------------------|
| `0 2 * * *` | 2 AM every day | Daily network scan |
| `0 3 * * 0` | 3 AM Sundays | Weekly vulnerability assessment |
| `0 1 1 * *` | 1 AM on 1st | Monthly full scan |
| `0 */6 * * *` | Every 6 hours | Continuous monitoring |
| `30 2 * * *` | 2:30 AM daily | Secondary daily scan |

Learn more: [API Reference - Cron Schedules](./API_REFERENCE.md#scheduling)

## Prerequisites Checklist

Before starting with KTtack, ensure you have:

- [ ] Go 1.24.0+ installed
- [ ] Docker 17.03+ installed
- [ ] kubectl 1.11.3+ installed
- [ ] Access to Kubernetes 1.11.3+ cluster
- [ ] Cluster admin privileges
- [ ] Container registry access (Docker Hub, GitHub, GCR, etc.)

See [Installation Guide - Prerequisites](./INSTALLATION_GUIDE.md#prerequisites-verification) for verification steps.

## Getting Help

### Documentation
- Search through documentation files above
- Check [Getting Started Troubleshooting](./GETTING_STARTED.md#troubleshooting)
- Review [Installation Troubleshooting](./INSTALLATION_GUIDE.md#troubleshooting-installation)

### Community
- Visit [GitHub Repository](https://github.com/kttack/kttack)
- Open an issue or discussion
- Check existing issues for similar problems

### Debugging
1. Check KTtack controller logs: `kubectl logs -n kttack-system deployment/kttack-controller-manager`
2. Check pod events: `kubectl describe pod -n <namespace> <pod-name>`
3. Review resource status: `kubectl describe securityattack -n <namespace> <name>`

## Contributing to Documentation

Documentation improvements are welcome! Areas that need contributions:

- [ ] Additional tool integration examples
- [ ] Advanced configuration recipes
- [ ] Performance tuning guides
- [ ] Multi-cluster setup guides
- [ ] Security hardening guides

See the main [README](../README.md#contributing) for contribution guidelines.

## Documentation Versions

This documentation is for KTtack `v1alpha1` API version.

### Versioning Strategy
- **v1alpha1**: Current experimental version (this documentation)
- **v1beta1**: Planned (will include migration guide)
- **v1**: Planned stable release

## Related Resources

- [Main README](../README.md) - Project overview
- [GitHub Repository](https://github.com/kttack/kttack) - Source code and issues
- [Kubebuilder Documentation](https://book.kubebuilder.io/) - Operator framework
- [Kubernetes Documentation](https://kubernetes.io/docs/) - Kubernetes concepts

---

**Last Updated**: January 2025
**KTtack Version**: v1alpha1
**Documentation Status**: Complete
