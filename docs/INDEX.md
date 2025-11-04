# KTtack Documentation Index

Welcome to the KTtack documentation! This directory contains comprehensive guides for installing, using, and understanding the KTtack Kubernetes Security Testing Framework.

## Documentation Overview

| Document | Lines | Purpose |
|----------|-------|---------|
| [Getting Started](./GETTING_STARTED.md) | 357 | Quick start guide - create your first security test in 5 minutes |
| [Installation Guide](./INSTALLATION_GUIDE.md) | 629 | Detailed installation methods, configuration, troubleshooting |
| [Architecture Guide](./ARCHITECTURE.md) | 529 | System design, components, data flows, extensibility |
| [Fluent Bit Sidecar](./FLUENT_BIT_SIDECAR.md) | 176 | Centralized logging setup with Loki |
| [API Reference](./API_REFERENCE.md) | 518 | CRD specifications and examples |

**Total Documentation**: 2,209 lines of comprehensive guides

---

## Quick Navigation

### 👤 New Users
Start here if you're new to KTtack:
1. [Getting Started](./GETTING_STARTED.md) - 5 minute introduction
2. [Installation Guide](./INSTALLATION_GUIDE.md) - Install KTtack on your cluster
3. Return to [Main README](../README.md) for overview

### 🏗️ Architects & DevOps
Understanding the system:
1. [Architecture Guide](./ARCHITECTURE.md) - System design and patterns
2. [API Reference](./API_REFERENCE.md) - CRD specifications
3. [Installation Guide](./INSTALLATION_GUIDE.md) - Deployment methods

### 🔧 System Administrators
Operational guides:
1. [Installation Guide](./INSTALLATION_GUIDE.md) - Multiple installation methods
2. [Fluent Bit Sidecar](./FLUENT_BIT_SIDECAR.md) - Configure logging
3. [Architecture Guide](./ARCHITECTURE.md) - Troubleshooting section

### 🚀 Security Engineers
Creating security tests:
1. [Getting Started](./GETTING_STARTED.md) - Create first test
2. [API Reference](./API_REFERENCE.md) - CRD examples and syntax
3. [Fluent Bit Sidecar](./FLUENT_BIT_SIDECAR.md) - Log aggregation

### 👨‍💻 Developers
Contributing to KTtack:
1. [Architecture Guide](./ARCHITECTURE.md) - System design
2. [API Reference](./API_REFERENCE.md) - CRD types and schemas
3. [Main README](../README.md) - Development section

---

## Document Descriptions

### 📖 Getting Started
**Audience**: New users  
**Time to read**: 10-15 minutes  
**Contains**:
- Prerequisites verification
- First SecurityAttack creation
- Monitoring execution
- Periodic scanning setup
- Troubleshooting basics
- Common use cases

**Best for**: Quick hands-on introduction with working examples

---

### 📋 Installation Guide
**Audience**: DevOps, System Administrators  
**Time to read**: 20-30 minutes  
**Contains**:
- 4 installation methods (pre-built, from source, Kustomize, Helm)
- Prerequisites verification
- Post-installation configuration
- Tool specifications setup
- Fluent Bit configuration
- Troubleshooting installation issues
- Upgrade and rollback procedures
- Complete uninstallation steps

**Best for**: Setting up KTtack in your environment

---

### 🏗️ Architecture Guide
**Audience**: Architects, Advanced Users, Developers  
**Time to read**: 30-40 minutes  
**Contains**:
- High-level system architecture with diagrams
- Core components breakdown
- CRD definitions and specifications
- Reconciliation flow
- Job and CronJob management
- Tool integration patterns
- Logging architecture
- Security and RBAC
- Complete data flow examples
- Extensibility guidelines
- Performance considerations
- Troubleshooting guide

**Best for**: Understanding how KTtack works internally

---

### 🔌 Fluent Bit Sidecar
**Audience**: DevOps, System Administrators  
**Time to read**: 10-15 minutes  
**Contains**:
- How Fluent Bit sidecar works
- Configuration requirements (Secret, ConfigMap)
- Deployment instructions
- Log label structure
- Debug vs. logging modes

**Best for**: Setting up centralized log aggregation to Loki

---

### 📚 API Reference
**Audience**: Security Engineers, Developers  
**Time to read**: 20-25 minutes  
**Contains**:
- SecurityAttack CRD specification
- Enumeration CRD specification
- Liveness CRD specification
- Field descriptions and validation rules
- Complete YAML examples
- Common patterns and use cases

**Best for**: Understanding CRD syntax and available options

---

## Common Tasks

### "I want to install KTtack"
→ Read [Installation Guide](./INSTALLATION_GUIDE.md)

### "I want to create a security test"
→ Read [Getting Started](./GETTING_STARTED.md)

### "I want to understand the system architecture"
→ Read [Architecture Guide](./ARCHITECTURE.md)

### "I need to set up centralized logging"
→ Read [Fluent Bit Sidecar](./FLUENT_BIT_SIDECAR.md)

### "I need CRD syntax examples"
→ Read [API Reference](./API_REFERENCE.md)

### "I'm having installation issues"
→ Read [Installation Guide - Troubleshooting](./INSTALLATION_GUIDE.md#troubleshooting-installation)

### "I'm having execution issues"
→ Read [Getting Started - Troubleshooting](./GETTING_STARTED.md#troubleshooting)

### "I want to extend KTtack with custom tools"
→ Read [Architecture Guide - Extensibility](./ARCHITECTURE.md#extensibility)

---

## Key Concepts

### Custom Resource Definitions (CRDs)
KTtack uses three main CRD types:
- **SecurityAttack**: One-time or periodic security tests
- **Enumeration**: Network and service enumeration operations
- **Liveness**: System health and availability checks

See [API Reference](./API_REFERENCE.md) for complete specifications.

### Reconciliation
KTtack controllers watch CRs and automatically create Kubernetes Jobs/CronJobs to execute security tests.

See [Architecture Guide - Controller Reconciliation Flow](./ARCHITECTURE.md#controller-reconciliation-flow) for details.

### Tool Integration
Tools are defined in a ConfigMap and executed in containerized pods with optional log aggregation via Fluent Bit.

See [Architecture Guide - Tool Integration](./ARCHITECTURE.md#tool-integration) for details.

### Centralized Logging
Optional Fluent Bit sidecar automatically ships logs to Loki for centralized access and querying.

See [Fluent Bit Sidecar Documentation](./FLUENT_BIT_SIDECAR.md) for setup.

---

## Installation Methods Quick Reference

| Method | Best For | Difficulty | Time |
|--------|----------|-----------|------|
| Pre-built Image | Production, Quick start | Easy | 5 min |
| Build from Source | Development, Custom | Medium | 15 min |
| Kustomize Bundle | Distribution | Easy | 10 min |
| Helm Chart | Advanced, GitOps | Medium | 10 min |

See [Installation Guide - Installation Methods](./INSTALLATION_GUIDE.md#installation-methods) for detailed instructions.

---

## CRD Quick Reference

### SecurityAttack
```yaml
apiVersion: kttack.io/v1alpha1
kind: SecurityAttack
metadata:
  name: example
  namespace: security-testing
spec:
  attackType: Enumeration
  target: "192.168.1.0/24"
  tool: nmap
  periodic: false
  args: ["-sV", "-A"]
```

### Enumeration
Similar to SecurityAttack, specialized for enumeration.

### Liveness
Health checks and availability probes.

See [API Reference](./API_REFERENCE.md) for complete specifications.

---

## Glossary

| Term | Definition |
|------|-----------|
| **CRD** | Custom Resource Definition - Kubernetes extension mechanism |
| **Reconciler** | Controller component that watches resources and takes action |
| **SecurityAttack** | Custom resource representing a security test |
| **Tool** | Security tool (Nmap, Nikto, Feroxbuster, etc.) |
| **Fluent Bit** | Log shipper that forwards logs to Loki |
| **Loki** | Log aggregation system (optional) |
| **Kustomize** | Kubernetes configuration management tool |

---

## Additional Resources

### External Links
- **GitHub Repository**: https://github.com/kttack/kttack
- **Kubernetes Documentation**: https://kubernetes.io/docs/
- **Kubebuilder Book**: https://book.kubebuilder.io/
- **Fluent Bit Documentation**: https://docs.fluentbit.io/
- **Loki Documentation**: https://grafana.com/docs/loki/latest/

### Related Documentation in Repository
- **Main README**: [../README.md](../README.md) - Project overview and quick links
- **License**: [../LICENSE](../LICENSE) - Apache 2.0 License
- **Contributing**: See Contributing section in [../README.md](../README.md)

---

## How to Use This Documentation

1. **Start with your use case** in the Quick Navigation section above
2. **Read the relevant document** for your role/task
3. **Use the glossary** if you encounter unfamiliar terms
4. **Reference specific sections** as needed
5. **Check troubleshooting sections** if you encounter issues

---

## Documentation Statistics

- **Total Documents**: 5 (plus this index)
- **Total Lines**: 2,209 (documentation only, excluding README)
- **Code Examples**: 50+
- **Diagrams**: 3 architectural diagrams
- **Tables**: 15+ reference tables

---

## Feedback & Contributions

If you find issues or have suggestions for improving the documentation:

1. Check the [GitHub Repository](https://github.com/kttack/kttack)
2. Open an issue with documentation feedback
3. Submit a pull request with improvements

---

**Last Updated**: November 2025  
**Documentation Version**: 1.0  
**KTtack Version Compatibility**: v1.0+
