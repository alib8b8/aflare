# Privacy Policy Compliance Audit

## Description
A comprehensive privacy policy compliance audit workflow that evaluates privacy policies against global regulations including GDPR, CCPA, and PIPEDA. The workflow fetches policy text, extracts data practices, audits against multiple regulatory frameworks, calculates compliance scores, and generates detailed remediation reports.

## Usage Example
```yaml
workflow: legal/privacy-policy-audit
params:
  policy_url: "https://example.com/privacy"
  regulations: ["GDPR", "CCPA", "PIPEDA"]
  industry: "healthcare"
  output_path: "output/privacy_audit.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| policy_url | string | No | - | URL of the privacy policy to audit |
| policy_text | string | No | - | Full text of the privacy policy (alternative) |
| regulations | array | No | [GDPR, CCPA, PIPEDA] | List of regulations to audit against |
| industry | string | No | general | Industry sector for additional checks |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/privacy_audit_report.md | Output file path |

## Nodes Used
- **http_request** (fetch_policy): Fetches privacy policy from URL
- **agent** (extract_data_practices): Extracts data collection and processing practices
- **agent** (audit_compliance): Audits practices against selected regulations
- **code_interpreter** (score_calculate): Calculates compliance scores and statistics
- **file_write** (save_report): Saves the audit report

## Category
legal