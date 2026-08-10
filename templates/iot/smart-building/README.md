# Smart Building

Smart building automation and monitoring with HVAC, lighting, access control, occupancy tracking, and energy optimization. Analyzes efficiency, applies automated optimizations, and provides AI-powered building management recommendations.

## Usage Example

```yaml
params:
  building_id: "building-001"
  zones: "floor-1,floor-2,floor-3"
  subsystems: "hvac,lighting,access,energy"
  occupancy_schedule: '{"weekday":{"start":"08:00","end":"18:00"},"weekend":{"start":"10:00","end":"16:00"}}'
  building_api: "https://api.building.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| building_id | string | "" | Building identifier |
| zones | string | "" | Comma-separated building zones or floors |
| subsystems | string | hvac,lighting,access,energy | Building subsystems to monitor |
| occupancy_schedule | string | {} | JSON occupancy schedule configuration |
| building_api | string | https://api.building.local/v1 | Smart building management API |

## Nodes Used

- **http_request** (fetch_building_state): Fetches current building subsystem state
- **code_interpreter**: Analyzes building efficiency and occupancy patterns
- **agent**: AI-powered building optimization recommendations
- **http_request** (apply_automation): Applies automated building optimization actions
- **file_write**: Saves building monitoring and optimization report

## Category

iot