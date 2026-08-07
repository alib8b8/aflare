# Tenant Isolation

aflare supports multi-tenant deployments with resource isolation between tenants.

## Overview

Tenant isolation allows you to:

- Run multiple independent workflows for different users/organizations
- Enforce resource quotas per tenant
- Isolate secrets and data between tenants
- Manage tenant lifecycle (create, delete, configure)

## Quick Start

### Create a Tenant

```bash
# Create tenant with default configuration
aflare tenant create --id acme --name "Acme Corp"

# Create tenant with custom quotas
aflare tenant create \
  --id startup \
  --name "Startup Inc" \
  --max-workflows 10 \
  --max-concurrency 5 \
  --max-storage 104857600
```

### List Tenants

```bash
aflare tenant list
```

### Get Tenant Info

```bash
aflare tenant info acme
```

### Delete a Tenant

```bash
aflare tenant delete acme
```

## Resource Isolation

### Directory Structure

Each tenant gets isolated directories for resources:

```
~/.aflare/
├── tenants/
│   ├── acme/
│   │   ├── workflows/        # Tenant-specific workflows
│   │   ├── history/          # Execution history
│   │   └── secrets.enc      # Encrypted secrets
│   └── startup/
│       ├── workflows/
│       ├── history/
│       └── secrets.enc
└── system/                   # System tenant resources
```

### Resource Types

| Resource | Description | Isolation Level |
|----------|-------------|-----------------|
| **Workflows** | YAML workflow files | Complete isolation |
| **History** | Execution logs and results | Complete isolation |
| **Secrets** | Encrypted secret storage | Complete isolation |

## Quota Management

### Configuring Quotas

```bash
# Set quotas when creating
aflare tenant create \
  --id enterprise \
  --name "Enterprise Ltd" \
  --max-workflows 100 \
  --max-concurrency 50 \
  --max-storage 1073741824
```

### Quota Limits

| Quota | Description | Default |
|-------|-------------|---------|
| `max_workflows` | Maximum number of workflows | Unlimited (0) |
| `max_concurrency` | Maximum concurrent executions | Unlimited (0) |
| `max_storage_bytes` | Maximum storage in bytes | Unlimited (0) |

### Checking Quota Usage

```bash
aflare tenant quota acme

# Output:
# Tenant: acme
# Workflows: 5/10
# Concurrent: 2/5
# Storage: 52428800/104857600 bytes
```

## Access Control

### Tenant Validation

The tenant manager validates that:

1. Tenant ID contains only alphanumeric characters and underscores
2. Tenant ID is between 1-64 characters
3. Resource paths belong to the tenant's directory
4. Quotas are not exceeded

### System Tenant

The system tenant (empty ID) has special privileges:

- No quota restrictions
- Access to all resources
- Cannot be deleted
- Used for system-level operations

### Context-Based Isolation

Tenant isolation is enforced via context:

```go
import (
    "github.com/alib8b8/aflare/internal/tenant"
)

// Set tenant in context
ctx := tenant.WithTenant(context.Background(), "acme")

// Get tenant from context
tenantID := tenant.GetTenantID(ctx)
```

## Use Cases

### SaaS Deployment

```bash
# Create tenants for each customer
aflare tenant create --id customer-a --name "Customer A"
aflare tenant create --id customer-b --name "Customer B"

# Deploy workflows per tenant
aflare run --tenant customer-a workflow-a.yaml
aflare run --tenant customer-b workflow-b.yaml
```

### Multi-Team Environment

```bash
# Create team tenants
aflare tenant create --id engineering --name "Engineering"
aflare tenant create --id marketing --name "Marketing"

# Set appropriate quotas
aflare tenant config engineering --max-workflows 50 --max-concurrency 20
aflare tenant config marketing --max-workflows 20 --max-concurrency 10
```

### Development/Testing

```bash
# Create isolated environments
aflare tenant create --id dev --name "Development"
aflare tenant create --id staging --name "Staging"
aflare tenant create --id prod --name "Production"

# Run workflows in isolation
aflare run --tenant dev test-workflow.yaml
```

## Security

### Data Protection

- Each tenant's secrets are encrypted with a separate key
- File permissions are set to `0750` (owner read/write, group read)
- Resource path validation prevents directory traversal
- Tenant data is deleted when tenant is removed

### Audit Trail

- All tenant operations are logged
- Quota violations are recorded
- Access attempts to unauthorized resources are tracked

## API Integration

### REST API

```bash
# Create tenant
curl -X POST http://localhost:8090/api/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "id": "new-tenant",
    "name": "New Tenant",
    "max_workflows": 20,
    "max_concurrency": 10,
    "max_storage_bytes": 52428800
  }'

# List tenants
curl http://localhost:8090/api/tenants

# Delete tenant
curl -X DELETE http://localhost:8090/api/tenants/new-tenant
```

### Programmatic Usage

```go
import (
    "github.com/alib8b8/aflare/internal/tenant"
)

// Create tenant manager
tm := tenant.NewTenantManager("/path/to/storage")

// Create tenant
config := tenant.TenantConfig{
    MaxWorkflows:    10,
    MaxConcurrency:  5,
    MaxStorageBytes: 104857600,
}
t, err := tm.CreateTenant("acme", "Acme Corp", config)

// Check quota
usage := tenant.QuotaUsage{
    WorkflowCount: 5,
    RunningCount:  2,
    StorageBytes:  52428800,
}
err := tm.CheckQuota("acme", usage)

// Validate access
err := tm.ValidateAccess("acme", "/path/to/tenant/workflow.yaml")
```

## Troubleshooting

### Tenant Not Found

1. Verify tenant exists: `aflare tenant list`
2. Check tenant ID spelling
3. Ensure tenant was created successfully

### Quota Exceeded

1. Check current usage: `aflare tenant quota <id>`
2. Increase quota: `aflare tenant config <id> --max-workflows 100`
3. Delete unused workflows

### Permission Denied

1. Verify file permissions
2. Check if resource belongs to the tenant
3. Ensure system tenant is used for cross-tenant operations

## Best Practices

1. **Use Descriptive IDs**: Use meaningful tenant IDs for easy identification
2. **Set Reasonable Quotas**: Prevent resource abuse with appropriate limits
3. **Regular Cleanup**: Remove unused tenants to free resources
4. **Backup Regularly**: Backup tenant data before deletion
5. **Monitor Usage**: Track quota usage to anticipate scaling needs
6. **Isolate Sensitive Data**: Use separate tenants for sensitive workloads
