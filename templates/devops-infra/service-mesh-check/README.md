# Service Mesh Health Check

> Check Istio/Service Mesh configuration and health

## Description

This workflow template provides a ready-to-use solution for check istio/service mesh configuration and health.

## Usage

```bash
aflare install devops-infra/service-mesh-check
aflare run service-mesh-check/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| - | Check workflow.yaml for configurable parameters | - |

## Nodes Used

- execute - Execute kubectl/istioctl commands for diagnostics
- agent - AI agent node for intelligent processing
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

devops-infra