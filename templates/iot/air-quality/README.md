# Air Quality

Air quality monitoring and reporting with pollutant analysis (PM2.5, PM10, O3, NO2, SO2, CO), AQI calculation using US EPA or China MEP standards, health advisories, and AI-powered trend prediction.

## Usage Example

```yaml
params:
  monitor_ids: "aq-001,aq-002"
  pollutants: "pm25,pm10,o3,no2,so2,co"
  aqi_standard: "us_epa"
  reporting_interval: "hourly"
  aq_api: "https://api.airquality.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| monitor_ids | string | "" | Comma-separated air quality monitor IDs |
| pollutants | string | pm25,pm10,o3,no2,so2,co | Pollutants to monitor |
| aqi_standard | string | us_epa | AQI calculation standard - us_epa or cn_mep |
| reporting_interval | string | hourly | Reporting interval |
| aq_api | string | https://api.airquality.local/v1 | Air quality monitoring API |

## Nodes Used

- **http_request**: Fetches air quality readings from monitors
- **code_interpreter**: Calculates AQI and categorizes air quality levels
- **agent**: AI-powered health advisory based on air quality
- **file_write**: Publishes air quality report

## Category

iot