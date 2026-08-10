# Smart Home Automation

Smart home automation rule engine that evaluates device state against user-defined rules, validates actions with AI, dispatches commands to devices, and logs execution for audit purposes.

## Usage Example

```yaml
params:
  home_id: "home-001"
  rules_config: '[{"id":"r1","name":"Night Light","condition":{"type":"threshold","device_id":"motion_sensor_1","property":"motion","operator":"==","value":1},"actions":[{"device_id":"light_1","command":"turn_on","params":{"brightness":30}}]}]'
  device_api_endpoint: "https://api.smarthome.local/v1"
  execution_mode: "live"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| home_id | string | "" | Unique identifier for the smart home |
| rules_config | string | "" | JSON string of automation rules |
| device_api_endpoint | string | https://api.smarthome.local/v1 | API endpoint for device control |
| execution_mode | string | simulate | Execution mode - simulate or live |

## Nodes Used

- **http_request** (fetch_state): Fetches current device state from home API
- **code_interpreter**: Evaluates automation rules against device state
- **agent**: AI-powered action validation and conflict detection
- **http_request** (dispatch_commands): Sends validated commands to devices
- **file_write**: Logs automation execution for audit trail

## Category

iot