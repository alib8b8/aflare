# Industrial PLC

Industrial PLC data collection and monitoring with multi-protocol support (Modbus, Profinet, Ethernet/IP, OPC UA). Processes signals with scaling and offset, detects alarms, and provides AI-powered operational analysis.

## Usage Example

```yaml
params:
  plc_ids: "plc-001,plc-002"
  protocol: "modbus"
  register_map: '{"temperature":{"address":40001,"scale":0.1,"unit":"°C","alarm":{"high":80,"low":0,"severity":"critical"}},"pressure":{"address":40002,"scale":1,"unit":"bar"}}'
  poll_interval_ms: 1000
  plc_api: "https://api.plc.local/v1"
```

## Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| plc_ids | string | "" | Comma-separated PLC device IDs |
| protocol | string | modbus | PLC protocol - modbus, profinet, ethernet_ip, or opc_ua |
| register_map | string | {} | JSON register mapping configuration |
| poll_interval_ms | integer | 1000 | Polling interval in milliseconds |
| plc_api | string | https://api.plc.local/v1 | PLC data collection API |

## Nodes Used

- **http_request**: Collects real-time data from PLC devices
- **code_interpreter**: Processes signals with scaling, offset, and alarm detection
- **agent**: AI-powered operational analysis
- **file_write**: Logs PLC data collection results

## Category

iot