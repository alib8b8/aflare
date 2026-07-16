# Troubleshooting Guide

This guide provides common error codes, error messages, and their solutions.

## Error Code Reference

### Workflow Execution Errors

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| WF001 | `node '%s' not found in registry` | The specified node doesn't exist or isn't registered | Check the node name spelling, ensure the node is installed, run `llm-box nodes list` to see available nodes |
| WF002 | `step %d (%s) failed: %w` | A step execution failed | Check the step's input and parameters, verify credentials, review node-specific error messages |
| WF003 | `workflow timed out during retry delay` | Workflow exceeded the timeout limit while waiting for retry | Increase `max_timeout` in workflow config, reduce retry count, optimize step execution time |
| WF004 | `condition evaluation failed: %w` | A condition expression couldn't be evaluated | Check the condition syntax, ensure referenced variables exist |
| WF005 | `expression evaluation failed: %w` | An expression like `{{step.0}}` couldn't be evaluated | Verify step indices/names exist, check variable references |
| WF006 | `too many parallel steps (%d, max %d)` | Exceeded maximum parallel steps limit (50) | Reduce the number of parallel steps |
| WF007 | `invalid workflow name: %s` | Workflow name contains invalid characters | Use only alphanumeric characters, hyphens, and underscores |

### Node-Specific Errors

#### HTTP/Network Errors

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| ND001 | `failed to fetch URL: %w` | Network request failed | Check network connectivity, verify URL, check firewall rules |
| ND002 | `HTTP request failed: %d %s` | HTTP request returned non-200 status | Check API endpoint, verify authentication, check rate limits |
| ND003 | `connection timeout` | Connection to remote server timed out | Increase timeout parameter, check server availability, reduce network latency |
| ND004 | `API key invalid or missing` | Authentication failed | Verify API key is correct, check secrets configuration, ensure `LLM_BOX_SECRETS_PASSWORD` is set |

#### File Operations

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| ND005 | `permission denied` | Insufficient file permissions | Check file/directory permissions, run with appropriate privileges |
| ND006 | `file not found: %s` | Specified file doesn't exist | Verify file path, check spelling, ensure file exists |
| ND007 | `invalid mode: %s (supported: write, append)` | Invalid file write mode | Use only `write` or `append` for the mode parameter |
| ND008 | `file too large` | File exceeds size limit | Reduce file size, check `MaxFileSize` constant |

#### LLM/AI Errors

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| ND009 | `model not found: %s` | Specified model doesn't exist | Verify model name, check provider availability, ensure model is supported |
| ND010 | `rate limit exceeded` | API rate limit reached | Wait and retry, increase rate limit with provider, implement caching |
| ND011 | `insufficient quota` | API usage quota exhausted | Check provider billing, increase quota, reduce usage |
| ND012 | `model unavailable` | Model is temporarily unavailable | Try again later, use a different model, check provider status |

#### Execute Node Errors

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| ND013 | `command not allowed in safe mode` | Attempted to run execute node in safe mode | Disable safe mode with `--safe-mode=false` or use allowlist |
| ND014 | `command not in allowlist` | Command not in the allowlist | Add command to allowlist or disable allowlist mode |
| ND015 | `command execution timed out` | Command exceeded timeout | Increase timeout parameter, optimize command |
| ND016 | `shell injection detected` | Command contains dangerous characters | Remove shell metacharacters (`;`, `|`, `&`, `` ` ``), use parameterized commands |

### YAML/Parsing Errors

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| YML001 | `only .yaml and .yml workflow files are allowed` | Invalid file extension | Rename file to use `.yaml` or `.yml` extension |
| YML002 | `invalid workflow file path: %w` | File path is invalid | Verify path, check for special characters |
| YML003 | `YAML parse error: %w` | Invalid YAML syntax | Check YAML formatting, ensure proper indentation, use YAML validator |
| YML004 | `invalid filename: %s` | Invalid characters in filename | Use only alphanumeric characters and underscores |
| YML005 | `missing required field: %s` | Required field is missing | Add the required field to the workflow YAML |

### Secrets Management Errors

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| SEC001 | `secrets password not set - set LLM_BOX_SECRETS_PASSWORD environment variable` | Master password not configured | Set `LLM_BOX_SECRETS_PASSWORD` environment variable |
| SEC002 | `invalid secret type: %s` | Unknown secret type | Use only `normal` or `secret` type |
| SEC003 | `file too short: invalid format` | Secrets file is corrupted or empty | Restore from backup, recreate secrets file |
| SEC004 | `failed to decrypt secrets: %w` | Incorrect master password | Verify `LLM_BOX_SECRETS_PASSWORD` is correct |
| SEC005 | `secret not found: %s/%s` | Requested secret doesn't exist | Add the secret using `llm-box secrets add` |

### Scheduler Errors

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| SCH001 | `invalid cron expression: %w` | Invalid cron syntax | Check cron expression format, use 5 fields: `minute hour day month weekday` |
| SCH002 | `task with id %q already exists` | Duplicate task ID | Use a different task ID or remove the existing task |
| SCH003 | `task with id %q not found` | Task doesn't exist | Check task ID, list tasks with `llm-box schedule --list` |
| SCH004 | `invalid step value %q` | Invalid step value in cron expression | Step must be a positive integer |
| SCH005 | `value %d out of range [%d, %d]` | Cron field value is out of bounds | Use valid ranges (minute: 0-59, hour: 0-23, etc.) |

### Distributed Execution Errors

| Error Code | Error Message | Cause | Solution |
|------------|---------------|-------|----------|
| DST001 | `invalid port: %s` | Invalid port number | Use numeric port between 1-65535 |
| DST002 | `invalid coordinator URL: %s` | Invalid coordinator address | Verify URL format, ensure coordinator is running |
| DST003 | `authentication failed` | Invalid auth token | Ensure auth tokens match between coordinator and workers |
| DST004 | `no available workers` | No workers registered or all at capacity | Start more workers, increase worker capacity |
| DST005 | `heartbeat timeout` | Worker didn't respond | Check worker health, verify network connectivity |

## Common Issues and Solutions

### Issue: Workflow fails with "node not found"

**Symptom**: `node 'openai' not found in registry`

**Solution**:
1. Verify the node name is spelled correctly
2. Check if the node is registered: `llm-box nodes list`
3. Ensure the node is built into the binary or installed as a plugin
4. If using custom nodes, ensure they're in the plugins directory

### Issue: API key authentication fails

**Symptom**: `API key invalid or missing`

**Solution**:
1. Set `LLM_BOX_SECRETS_PASSWORD` environment variable
2. Verify the secret exists: `llm-box secrets list <group>`
3. Check the reference syntax: `{{secret.group.key}}`
4. Ensure the key value is correct

### Issue: Network timeout

**Symptom**: `connection timeout` or `HTTP request failed: 0 `

**Solution**:
1. Verify network connectivity to the target server
2. Increase timeout parameter: `timeout: "30s"`
3. Check firewall rules
4. Try accessing the URL directly with curl

### Issue: YAML parsing error

**Symptom**: `YAML parse error: line 10: could not find expected ':'`

**Solution**:
1. Check YAML indentation (use spaces, not tabs)
2. Ensure colons are followed by spaces
3. Validate YAML with an online validator
4. Check for special characters that need escaping

### Issue: File permission denied

**Symptom**: `permission denied` when writing to file

**Solution**:
1. Check file/directory permissions: `ls -la`
2. Change permissions: `chmod 755 directory`
3. Run llm-box with appropriate user permissions
4. Write to a directory you have access to

### Issue: Execute node command not allowed

**Symptom**: `command not allowed in safe mode`

**Solution**:
1. Disable safe mode: `llm-box --safe-mode=false run workflow.yaml`
2. Or set environment variable: `LLM_BOX_SAFE_MODE=0`
3. For production, use allowlist mode instead

### Issue: Secrets not available

**Symptom**: `secrets not available - use 'llm-box secrets add' to store secrets first`

**Solution**:
1. Set `LLM_BOX_SECRETS_PASSWORD` environment variable
2. Add secrets via CLI: `llm-box secrets add --group llm --key openai --value sk-...`
3. Verify secrets are stored: `llm-box secrets list llm`

### Issue: Cron expression invalid

**Symptom**: `invalid cron expression: hour field: expected 5 fields, got 6`

**Solution**:
1. Use standard 5-field cron syntax
2. Example: `0 9 * * *` (daily at 9 AM)
3. Avoid 6-field cron formats
4. Test with: `llm-box schedule --cron "0 9 * * *" --validate`

## Debugging Tips

### Enable Verbose Logging

```bash
# Enable debug logging
LLM_BOX_LOG_LEVEL=debug llm-box run workflow.yaml

# View detailed logs
tail -f ~/.llm-box/logs/audit.log
```

### Use Dry Run Mode

```bash
# Preview commands without executing
llm-box run --dry-run workflow.yaml
```

### Check Step Output

```bash
# Run with verbose output to see each step's input/output
llm-box run --verbose workflow.yaml
```

### Validate Workflow Syntax

```bash
# Validate YAML syntax without running
llm-box validate workflow.yaml
```

### Test Individual Nodes

```bash
# Test a node directly
llm-box node test http_request --params '{"url":"https://example.com"}'
```

## Log Analysis

### Common Log Patterns

```bash
# Search for errors
grep -i "error\|failed" ~/.llm-box/logs/audit.log

# Search for specific workflow
grep "my-workflow" ~/.llm-box/logs/audit.log

# Search for API-related errors
grep -i "api\|auth\|token" ~/.llm-box/logs/audit.log

# View recent logs
tail -n 100 ~/.llm-box/logs/audit.log
```

### Log Field Reference

| Field | Description |
|-------|-------------|
| `time` | ISO 8601 timestamp |
| `level` | Log level (debug, info, warn, error) |
| `command` | Command executed |
| `workflow` | Workflow name |
| `step` | Step index |
| `node` | Node name |
| `duration` | Execution duration |
| `error` | Error message (redacted) |

## Getting Help

If you can't resolve an issue:

1. Check the [README](README.md) for general usage information
2. Review the [docs/](docs/) directory for detailed guides
3. Check the [FAQ](FAQ.md) for common questions
4. Look at [SECURITY.md](SECURITY.md) for security-related issues
5. Search GitHub issues for similar problems
6. Create a new issue with:
   - Error message
   - Workflow YAML (redact secrets)
   - Steps to reproduce
   - Environment details (OS, llm-box version)
