# Netirk Configuration Guide

This guide explains how to configure Netirk for network monitoring, analysis, and alerting.

## Quick Start

1. **Generate an example configuration:**
   ```bash
   netirk validate --generate-example my-config.yaml
   ```

2. **Validate your configuration:**
   ```bash
   netirk validate my-config.yaml
   ```

3. **Start monitoring:**
   ```bash
   netirk monitor --targets-file my-config.yaml --interval 30s
   ```

## Configuration File Structure

Netirk uses YAML configuration files with the following main sections:

### Targets Section

The `targets` section defines what to monitor:

```yaml
targets:
  - url: "https://example.com"
    timeout: "30s"
    interval: "60s"
    expected_status: 200
    ssl_check: true
    headers:
      Authorization: "Bearer token"
```

#### Target Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `url` | string | Yes | - | Target URL (http, https, or tcp) |
| `timeout` | duration | No | 30s | Request timeout |
| `interval` | duration | No | 60s | Monitoring interval |
| `expected_status` | int | No | 200 | Expected HTTP status code |
| `ssl_check` | bool | No | false | Enable SSL certificate monitoring |
| `headers` | map | No | {} | Custom HTTP headers |

#### Supported URL Formats

- **HTTP/HTTPS:** `https://api.example.com/health`
- **TCP:** `tcp://database.example.com:5432`

#### Duration Format

Use Go duration strings:
- `30s` - 30 seconds
- `5m` - 5 minutes
- `1h` - 1 hour
- `500ms` - 500 milliseconds

### Alert Configuration (Optional)

```yaml
alert_config:
  enabled: true
  failure_threshold: 3
  ssl_warning_days: 30
  ssl_critical_days: 7
  webhook_url: "https://hooks.slack.com/services/..."
  email_recipients:
    - "admin@example.com"
  email_config:
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "alerts@example.com"
    password: "app-password"
    from_address: "alerts@example.com"
    use_starttls: true
```

#### Alert Fields

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | bool | Enable/disable alerting |
| `failure_threshold` | int | Consecutive failures before alert |
| `ssl_warning_days` | int | Days before SSL expiry to warn |
| `ssl_critical_days` | int | Days before SSL expiry for critical alert |
| `webhook_url` | string | Slack/webhook URL for notifications |
| `email_recipients` | []string | Email addresses for alerts |

## Command Reference

### Monitor Command

Monitor targets continuously:

```bash
# Basic monitoring
netirk monitor --targets-file config.yaml

# With custom interval
netirk monitor --targets-file config.yaml --interval 30s

# Limited duration
netirk monitor --targets-file config.yaml --duration 1h

# Save results
netirk monitor --targets-file config.yaml --output-file results.json

# With alerts
netirk monitor --targets-file config.yaml --alert-threshold 3
```

#### Monitor Flags

| Flag | Type | Description |
|------|------|-------------|
| `--targets-file` | string | Configuration file path (required) |
| `--interval` | duration | Monitoring interval (default: 30s) |
| `--duration` | duration | Total monitoring duration |
| `--output-file` | string | Save results to file |
| `--output-format` | string | Output format (json, csv, html, prometheus) |
| `--alert-threshold` | int | Failure threshold for alerts |
| `--alert-webhook` | string | Webhook URL for alerts |
| `--alert-email` | string | Email address for alerts |

### Analyze Command

Analyze historical monitoring data:

```bash
# Basic analysis
netirk analyze --data-file monitoring.json

# With trends and anomalies
netirk analyze --data-file monitoring.json --show-trends

# Compare time periods
netirk analyze --data-file monitoring.json --compare-periods

# JSON output
netirk analyze --data-file monitoring.json --output json
```

#### Analyze Flags

| Flag | Type | Description |
|------|------|-------------|
| `--data-file` | string | JSON data file to analyze (required) |
| `--output` | string | Output format (json or human-readable) |
| `--target` | string | Analyze specific target only |
| `--show-trends` | bool | Include trend analysis |
| `--compare-periods` | bool | Compare time periods |

### Validate Command

Validate configuration files:

```bash
# Validate configuration
netirk validate config.yaml

# Generate example
netirk validate --generate-example example.yaml

# Verbose validation
netirk validate config.yaml --verbose
```

## Example Configurations

### Simple Web Monitoring

```yaml
targets:
  - url: "https://google.com"
  - url: "https://github.com"
  - url: "https://api.example.com"
    timeout: "10s"
    expected_status: 200
```

### API Monitoring with Authentication

```yaml
targets:
  - url: "https://api.example.com/health"
    timeout: "15s"
    interval: "30s"
    expected_status: 200
    headers:
      Authorization: "Bearer your-token"
      User-Agent: "Netirk Monitor"
```

### Database Connection Monitoring

```yaml
targets:
  - url: "tcp://database.example.com:5432"
    timeout: "5s"
    interval: "60s"
  - url: "tcp://redis.example.com:6379"
    timeout: "3s"
    interval: "30s"
```

### SSL Certificate Monitoring

```yaml
targets:
  - url: "https://example.com"
    ssl_check: true
    interval: "3600s"  # Check hourly
  - url: "https://api.example.com"
    ssl_check: true
    interval: "3600s"

alert_config:
  enabled: true
  ssl_warning_days: 30
  ssl_critical_days: 7
  email_recipients:
    - "ssl-admin@example.com"
```

## Best Practices

### Monitoring Intervals

- **Web services:** 30s - 5m
- **APIs:** 1m - 10m
- **Databases:** 30s - 2m
- **SSL certificates:** 1h - 24h

### Timeouts

- **Fast APIs:** 5s - 15s
- **Slow services:** 30s - 60s
- **Database connections:** 3s - 10s

### Alert Thresholds

- **Critical services:** 1-2 failures
- **Non-critical services:** 3-5 failures
- **Flaky services:** 5-10 failures

### SSL Monitoring

- **Warning:** 30 days before expiry
- **Critical:** 7 days before expiry
- **Check frequency:** Every 1-24 hours

## Troubleshooting

### Common Configuration Errors

1. **Invalid URL format:**
   ```
   Error: target 0: invalid URL 'example.com'
   Suggestion: URLs must include protocol (https://example.com)
   ```

2. **Invalid duration:**
   ```
   Error: target 0: invalid timeout format '30'
   Suggestion: Use duration strings like "30s", "1m", "5m"
   ```

3. **Invalid status code:**
   ```
   Error: target 0: expected_status must be between 100-599
   Suggestion: Use valid HTTP status codes like 200, 201, 404
   ```

### Validation Commands

```bash
# Check configuration syntax
netirk validate config.yaml

# Verbose validation with summary
netirk validate config.yaml --verbose

# Generate working example
netirk validate --generate-example working-config.yaml
```

### Getting Help

```bash
# General help
netirk help

# Command-specific help
netirk help monitor
netirk help analyze
netirk help validate

# Flag information
netirk monitor --help
netirk analyze --help
```

## Integration Examples

### CI/CD Pipeline

```bash
# Validate configuration in CI
netirk validate monitoring-config.yaml

# Run monitoring for deployment verification
netirk monitor --targets-file deploy-check.yaml --duration 5m --output-file deploy-results.json

# Analyze results
netirk analyze --data-file deploy-results.json --output json > analysis.json
```

### Cron Job Monitoring

```bash
# Hourly monitoring with results
0 * * * * /usr/local/bin/netirk monitor --targets-file /etc/netirk/config.yaml --duration 55m --output-file /var/log/netirk/monitoring-$(date +\%Y\%m\%d-\%H).json

# Daily analysis
0 6 * * * /usr/local/bin/netirk analyze --data-file /var/log/netirk/monitoring-$(date -d yesterday +\%Y\%m\%d-23).json --show-trends --output json > /var/log/netirk/daily-analysis.json
```

### Docker Integration

```dockerfile
FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY netirk /usr/local/bin/
COPY config.yaml /etc/netirk/
CMD ["netirk", "monitor", "--targets-file", "/etc/netirk/config.yaml"]
```

## Support

For additional help:
- Use `netirk help [command]` for command-specific help
- Use `netirk validate --generate-example` for working examples
- Check configuration with `netirk validate config.yaml --verbose`