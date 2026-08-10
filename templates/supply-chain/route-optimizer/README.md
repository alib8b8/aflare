# Route Optimizer

Multi-stop delivery route optimization using nearest-neighbor heuristic and distance matrix computation.

## Description

This workflow geocodes delivery addresses, computes a haversine distance matrix between all stops, applies a nearest-neighbor constructive heuristic to build optimized routes, and optionally splits the workload across multiple vehicles. AI generates a detailed execution plan with fuel estimates and contingency notes.

## Usage

```yaml
params:
  depot_address: "123 Warehouse Blvd, Chicago, IL"
  delivery_stops: '[{"address":"456 Oak St","window":"09:00-12:00"},{"address":"789 Pine Ave","window":"10:00-14:00"}]'
  vehicle_count: 2
  max_distance_km: 200
  maps_api_key: "your-api-key"
  output_file: "/tmp/route_plan.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| depot_address | string | yes | - | Starting depot/warehouse address |
| delivery_stops | string | yes | - | JSON array of delivery stop addresses with time windows |
| vehicle_count | integer | no | 1 | Number of available vehicles |
| max_distance_km | number | no | 200 | Maximum distance per route in kilometers |
| maps_api_key | string | yes | - | API key for geocoding and distance matrix service |
| output_file | string | no | /tmp/route_plan.json | Output file for optimized route plan |

## Nodes Used

- **http_request** - Geocodes delivery addresses to coordinates
- **code_interpreter** - Computes distance matrix and runs nearest-neighbor optimization
- **agent** - Generates route execution plan with fuel estimates and contingencies
- **file_write** - Saves optimized route plan to output file

## Category

supply-chain