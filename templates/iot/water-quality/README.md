# Water Quality

Water quality monitoring and alerting with multi-parameter analysis (pH, dissolved oxygen, turbidity, conductivity, temperature, TDS). Assesses quality against regulatory standards (drinking water, wastewater, aquaculture), detects contamination, and provides AI-powered treatment recommendations.

## Usage Example

```yaml
params:
  station_ids: "ws-001,ws-002"
  quality_parameters: "ph,dissolved_oxygen,turbidity,conductivity,temperature,tds"
  standards_profile: "drinking_water"
  sampling_interval: "1h"
  water_api: "https://api.water.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| station_ids | string | "" | Comma-separated water monitoring station IDs |
| quality_parameters | string | ph,dissolved_oxygen,turbidity,conductivity,temperature,tds | Water quality parameters |
| standards_profile | string | drinking_water | Standard profile - drinking_water, wastewater, industrial, aquaculture |
| sampling_interval | string | 1h | Sampling interval |
| water_api | string | https://api.water.local/v1 | Water quality monitoring API |

## Nodes Used

- **http_request**: Fetches water quality readings from monitoring stations
- **code_interpreter**: Assesses water quality against regulatory standards
- **agent**: AI-powered water contamination analysis
- **file_write**: Generates water quality compliance report

## Category

iot