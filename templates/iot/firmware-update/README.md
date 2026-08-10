# Firmware Update

OTA firmware update management with firmware validation, canary/phased/all-at-once rollout strategies, AI-powered rollout monitoring, and campaign audit logging. Supports staged rollouts with rollback capability.

## Usage Example

```yaml
params:
  firmware_version: "2.1.0"
  firmware_url: "https://cdn.example.com/firmware/v2.1.0.bin"
  target_devices: "device-001,device-002,device-003"
  rollout_strategy: "canary"
  canary_percentage: 10
  ota_api: "https://api.ota.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| firmware_version | string | "" | Firmware version to deploy (semver) |
| firmware_url | string | "" | URL to firmware binary |
| target_devices | string | "" | Comma-separated device IDs |
| rollout_strategy | string | canary | Rollout strategy - canary, phased, or all_at_once |
| canary_percentage | integer | 10 | Percentage of devices for canary rollout |
| ota_api | string | https://api.ota.local/v1 | OTA firmware management API |

## Nodes Used

- **code_interpreter**: Validates firmware version format and URL
- **http_request**: Creates OTA update campaign
- **agent**: AI-powered rollout monitoring and decision support
- **file_write**: Persists campaign record for audit

## Category

iot