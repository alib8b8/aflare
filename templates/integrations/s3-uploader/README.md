# S3 Uploader

Upload and manage files in AWS S3 buckets with automated manifest generation.

## Description

List local files ready for upload, use AI to determine optimal upload strategy and content types, list existing bucket contents, and generate a detailed upload manifest. Works with any AWS S3-compatible storage.

## Install

```bash
aflare install s3-uploader
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export S3_BUCKET="your-bucket-name"
export AWS_REGION="us-east-1"
```

## Usage

```bash
# Place files to upload in the uploads/ directory
mkdir -p uploads/
cp /path/to/files/* uploads/

# Run the workflow
aflare run templates/s3-uploader/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `aws_access_key` | AWS access key ID | Required |
| `aws_secret_key` | AWS secret access key | Required |
| `s3_bucket` | Target S3 bucket name | Required |
| `aws_region` | AWS region | Required |

## Nodes Used

- `execute` — List files in uploads directory
- `agent` — AI-powered upload strategy determination
- `http_request` — List S3 bucket contents
- `file_write` — Save upload manifest
- `notify` — Display confirmation

## Output

- `s3-upload-manifest.json` — Upload manifest with bucket contents and upload plan
- Terminal confirmation with bucket and region details

## Schedule

```bash
# Daily file sync at midnight
0 0 * * * aflare run /path/to/templates/s3-uploader/workflow.yaml
```

## Category

integrations