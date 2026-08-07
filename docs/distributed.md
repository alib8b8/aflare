# Distributed Execution

> **⚠️ Status: Not Implemented (Design Document Only)**
>
> The Coordinator/Worker architecture described below is a design proposal.
> No code exists under `internal/distributed/`, and the `aflare coordinator`
> / `aflare worker` CLI commands are not available. This document is retained
> as a design reference for future implementation.

aflare supports distributed workflow execution across multiple machines using a Coordinator/Worker architecture.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Coordinator                                │
│  ┌─────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │ Nodes   │  │ Tasks    │  │ Heartbeat│  │ Task Dispatcher  │ │
│  │ Registry│  │ Manager  │  │ Monitor  │  │ (Load Balancing) │ │
│  └────┬────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘ │
│       │            │             │                  │          │
└───────┼────────────┼─────────────┼──────────────────┼──────────┘
        │            │             │                  │
        ▼            ▼             ▼                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Network                                 │
└─────────────────────────────────────────────────────────────────┘
        │            │             │                  │
        ▼            ▼             ▼                  ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Workers                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ Worker Node  │  │ Worker Node  │  │ Worker Node  │          │
│  │ (Capacity:5) │  │ (Capacity:3) │  │ (Capacity:10)│          │
│  │ Host:node1   │  │ Host:node2   │  │ Host:node3   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Step 1: Start the Coordinator

```bash
# Start coordinator on default port (8090)
aflare coordinator --auth-token my-secret-token

# Start on custom port
aflare coordinator --port 8090 --auth-token my-secret-token
```

### Step 2: Start Workers

On each worker machine:

```bash
# Start worker with default capacity (5 tasks)
aflare worker --coordinator http://coordinator-host:8090 --auth-token my-secret-token

# Start with custom capacity
aflare worker --coordinator http://coordinator-host:8090 --auth-token my-secret-token --capacity 10 --port 8091
```

### Step 3: Submit Workflow

```bash
# Submit workflow for distributed execution
aflare run --distributed http://coordinator-host:8090 my-workflow.yaml
```

## Configuration

### Coordinator Options

| Option | Default | Description |
|--------|---------|-------------|
| `--port` | `8090` | HTTP port to listen on |
| `--auth-token` | (empty) | Authentication token for worker registration (required for production) |

### Worker Options

| Option | Default | Description |
|--------|---------|-------------|
| `--port` | `8091` | HTTP port to listen on |
| `--coordinator` | `http://localhost:8090` | Coordinator URL |
| `--auth-token` | (empty) | Authentication token matching coordinator |
| `--capacity` | `5` | Maximum concurrent tasks this worker can handle |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AFLARE_COORDINATOR` | Default coordinator URL |
| `AFLARE_AUTH_TOKEN` | Default authentication token |

## API Endpoints

### Coordinator API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/register` | POST | Register a worker node |
| `/api/heartbeat` | POST | Send worker heartbeat |
| `/api/nodes` | GET | List all registered nodes |
| `/api/task` | GET/POST/PUT | Task operations |
| `/api/tasks` | GET | List all tasks |
| `/api/execute` | POST | Submit workflow for execution |
| `/health` | GET | Health check |

### Worker API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/execute-step` | POST | Execute a workflow step |
| `/health` | GET | Health check |

## Workflow Execution Flow

1. **Submit**: User submits workflow YAML to Coordinator
2. **Parse**: Coordinator parses the workflow
3. **Assign**: For each step, Coordinator selects the best available worker using load balancing
4. **Execute**: Worker executes the step locally
5. **Report**: Worker reports results back to Coordinator
6. **Collect**: Coordinator collects all step results and returns final output

### Load Balancing

The Coordinator uses a simple but effective load balancing strategy:

1. Filter out offline nodes (no heartbeat for >30 seconds)
2. Filter out nodes at capacity
3. Select the node with the lowest current load

## Distributed Workflow Example

Workflows are written the same way as local workflows. The Coordinator handles distribution automatically.

```yaml
name: distributed-data-processing
vars:
  api_key: "{{secret.api.service}}"

steps:
  - node: http_request
    params:
      url: "https://api.example.com/data"
      headers: "Authorization: Bearer {{var.api_key}}"
    id: fetch_data

  - node: json_parse
    params:
      path: results
    input: fetch_data
    id: parse_data

  - node: agent
    params:
      provider: ollama
      model: llama3
    input: "Analyze this data: {{step.parse_data}}"
    id: analyze

  - node: file_write
    params:
      path: analysis-report.md
    input: analyze
    id: save_report
```

## Security Considerations

### Authentication

Always use the `--auth-token` flag in production. The token is validated on every request between Coordinator and Workers.

### Network Security

- Use HTTPS for production deployments (recommend reverse proxy like nginx with Let's Encrypt)
- Restrict firewalls to allow only Coordinator ↔ Worker communication
- Consider VPN for multi-cloud deployments

### Secrets Management

Each worker maintains its own secrets file. For distributed deployments:

**Option 1: Manual Sync (Recommended for small teams)**
```bash
# On coordinator
aflare secrets export > secrets.json

# On each worker
aflare secrets import < secrets.json
```

**Option 2: Environment Variables**
```bash
# Set on all workers
export AFLARE_SECRETS_PASSWORD="your-master-password"
```

**Option 3: Shared Secrets Volume (Docker/Kubernetes)**
- Mount a shared encrypted volume containing the secrets file
- Ensure proper file permissions (0600)

## Monitoring

### Check Coordinator Status

```bash
# List all registered nodes
curl -H "X-Auth-Token: my-secret-token" http://coordinator:8090/api/nodes

# List all tasks
curl -H "X-Auth-Token: my-secret-token" http://coordinator:8090/api/tasks

# Check task status
curl -H "X-Auth-Token: my-secret-token" "http://coordinator:8090/api/task?task_id=task-123"
```

### Logs

Logs are stored locally on each machine:

- Coordinator: `~/.aflare/logs/coordinator.log`
- Worker: `~/.aflare/logs/worker.log`

## Troubleshooting

### Worker Not Registering

1. Check Coordinator URL is correct
2. Verify auth tokens match
3. Check firewall rules allow traffic between worker and coordinator
4. Check logs: `cat ~/.aflare/logs/worker.log`

### Tasks Not Being Assigned

1. Ensure workers are registered: `curl http://coordinator:8090/api/nodes`
2. Check worker capacity is not exceeded
3. Verify heartbeat is being sent (every 10 seconds)

### Failed Steps

1. Check worker logs for error details
2. Verify the step node is installed on the worker
3. Check network connectivity from worker to external services

## High Availability

For production deployments requiring high availability:

1. **Multiple Coordinators**: Use a load balancer in front of multiple coordinators
2. **Worker Failover**: Workers automatically re-register if coordinator restarts
3. **Task Recovery**: Failed tasks are reassigned to other available workers

## Scaling Recommendations

| Deployment Size | Coordinator | Workers | Capacity |
|-----------------|-------------|---------|----------|
| Small (<100 tasks/day) | 1 | 2-3 | 5 each |
| Medium (<1000 tasks/day) | 1 | 5-10 | 10 each |
| Large (>1000 tasks/day) | 2+ (load balanced) | 10+ | 10-20 each |

## Performance Tips

1. **Local Execution**: For small workflows, use local execution (`aflare run`) instead of distributed
2. **Worker Placement**: Place workers near data sources to minimize network latency
3. **Capacity Planning**: Monitor worker load and adjust capacity as needed
4. **Heartbeat Interval**: Default is 10 seconds; reduce for high-latency networks

## Limitations

- Workflows execute sequentially (one step at a time)
- No automatic workflow state persistence across coordinator restarts
- No encryption for task data in transit (use HTTPS)
