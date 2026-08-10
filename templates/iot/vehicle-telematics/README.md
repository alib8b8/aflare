# Vehicle Telematics

Vehicle telematics data processing with GPS tracking, driver behavior analysis (speed, aggressive driving), geofence violation detection, and AI-powered fleet optimization insights including safety, fuel efficiency, and maintenance alerts.

## Usage Example

```yaml
params:
  vehicle_ids: "v-001,v-002,v-003"
  fleet_id: "fleet-alpha"
  metrics: "speed,fuel,location,engine_rpm,odometer"
  geo_fence: '{"bounds":{"min_lat":40.0,"max_lat":41.0,"min_lon":-74.0,"max_lon":-73.0}}'
  telematics_api: "https://api.telematics.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| vehicle_ids | string | "" | Comma-separated vehicle IDs |
| fleet_id | string | "" | Fleet identifier for group analytics |
| metrics | string | speed,fuel,location,engine_rpm,odometer | Telemetry metrics |
| geo_fence | string | {} | JSON geofence boundary definition |
| telematics_api | string | https://api.telematics.local/v1 | Vehicle telematics API endpoint |

## Nodes Used

- **http_request**: Fetches real-time telematics data from vehicles
- **code_interpreter**: Analyzes driving behavior and detects geofence violations
- **agent**: AI-powered fleet optimization insights
- **file_write**: Logs telematics data to persistent storage

## Category

iot