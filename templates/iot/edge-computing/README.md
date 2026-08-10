# Edge Computing

Edge computing workload distribution and task orchestration across IoT edge nodes. Queries node status, optimizes scheduling using round-robin, least-loaded, or latency-aware strategies, dispatches tasks, and provides AI-powered analysis.

## Usage Example

```yaml
params:
  edge_nodes: "edge-001,edge-002,edge-003"
  workload_type: "inference"
  task_payload: '{"model":"sensor-anomaly","chunks":[{"data":"batch1"},{"data":"batch2"}]}'
  scheduling_strategy: "least_loaded"
  edge_api: "https://api.edge.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| edge_nodes | string | "" | Comma-separated edge node IDs |
| workload_type | string | inference | Workload type - inference, preprocessing, aggregation, or filtering |
| task_payload | string | {} | JSON task payload to distribute |
| scheduling_strategy | string | round_robin | Scheduling strategy - round_robin, least_loaded, or latency_aware |
| edge_api | string | https://api.edge.local/v1 | Edge computing management API |

## Nodes Used

- **http_request** (query_node_status): Queries edge node status and load
- **code_interpreter**: Optimizes workload distribution based on strategy
- **http_request** (dispatch_tasks): Dispatches tasks to assigned edge nodes
- **agent**: AI-powered workload distribution analysis
- **file_write**: Persists workload distribution record

## Category

iot