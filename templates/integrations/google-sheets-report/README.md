# Google Sheets Report

Generate and update reports in Google Sheets from various data sources.

## Description

Fetch data from Google Sheets and external APIs, use AI to analyze and generate insights, then append formatted reports back to the spreadsheet. Ideal for automated weekly reporting.

## Install

```bash
aflare install google-sheets-report
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export GOOGLE_SHEETS_API_KEY="your-google-api-key"
export SPREADSHEET_ID="your-spreadsheet-id"
export WEATHER_API_KEY="your-openweathermap-key"
```

## Usage

```bash
aflare run templates/google-sheets-report/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `google_api_key` | Google Sheets API key | Required |
| `spreadsheet_id` | Target spreadsheet ID | Required |
| `weather_api_key` | OpenWeatherMap API key | Optional |

## Nodes Used

- `http_request` — Read from Google Sheets, fetch weather data, append report
- `agent` — AI-powered data analysis and report generation
- `file_write` — Save report to markdown
- `notify` — Display confirmation

## Output

- `google-sheets-report.md` — Generated report markdown
- New row appended to the "Report" sheet in the spreadsheet

## Schedule

```bash
# Weekly report every Monday
0 9 * * 1 aflare run /path/to/templates/google-sheets-report/workflow.yaml
```

## Category

integrations