# Device Registry

IoT device registry and metadata management with CRUD operations, device validation, lifecycle tracking, and AI-powered compliance analysis. Supports register, update, query, and decommission operations.

## Usage Example

```yaml
params:
  registry_api: "https://api.iot-registry.local/v1"
  operation: "register"
  device_payload: '{"device_id":"sensor-temp-001","device_type":"sensor","manufacturer":"ACME","model":"T2000","firmware":"2.1.0"}'
  device_filter: "{}"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| registry_api | string | https://api.iot-registry.local/v1 | Device registry API endpoint |
| operation | string | register | Operation type - register, update, query, or decommission |
| device_payload | string | {} | JSON device payload for register/update operations |
| device_filter | string | {} | JSON filter criteria for query operations |

## Nodes Used

- **code_interpreter**: Validates device metadata completeness and format
- **http_request**: Executes device registry CRUD operations
- **agent**: AI-powered device lifecycle analysis and compliance check
- **file_write**: Persists device registry record

## Category

iot