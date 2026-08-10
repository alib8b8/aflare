# K8s Troubleshooter

> Diagnose and fix Kubernetes issues from error logs

## Description

This workflow template provides a ready-to-use solution for diagnose and fix kubernetes issues from error logs.

## Usage

```bash
aflare install devops-infra/k8s-troubleshooter
aflare run k8s-troubleshooter/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| namespace | Target Kubernetes namespace | Yes |
| resource_type | Resource type (pod, deployment, service, etc.) | Yes |
| resource_name | Name of the resource to troubleshoot | Yes |
| symptom | Description of the observed symptom | Yes |
| error_logs | Raw error logs (optional) | No |

## Nodes Used

- execute - Execute kubectl commands for cluster diagnostics
- agent - AI agent node for intelligent processing
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

devops-infra