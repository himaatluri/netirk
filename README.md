# Netirk

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fhimasagaratluri%2Fnetirk.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fhimasagaratluri%2Fnetirk?ref=badge_shield) [![Go Report Card](https://goreportcard.com/badge/github.com/himasagaratluri/netirk)](https://goreportcard.com/report/github.com/himasagaratluri/netirk)

[dev.to - Introducing NetIrk](https://dev.to/himaatluri/introducing-netirk-a-lightweight-cli-tool-for-high-level-network-insights-5e4p)

```(shell)
               _    _        _    
  _ __    ___ | |_ (_) _ __ | | __
 | '_ \  / _ \| __|| || '__|| |/ /
 | | | ||  __/| |_ | || |   |   < 
 |_| |_| \___| \__||_||_|   |_|\_\
```

A lightweight CLI tool for comprehensive network monitoring, analysis, and troubleshooting. Netirk provides real-time monitoring capabilities, historical data analysis, and multiple export formats for integration with monitoring systems.

## Features

### Core Capabilities
- **Real-time Network Monitoring**: Continuous monitoring of HTTP, HTTPS, and TCP targets
- **SSL Certificate Monitoring**: Track certificate expiration and validity
- **Historical Data Analysis**: Analyze monitoring data with trend detection and anomaly identification
- **Multiple Export Formats**: JSON, CSV, HTML reports with charts, and Prometheus metrics
- **Alerting System**: Webhook and email notifications for failures and recoveries
- **Performance Optimization**: Efficient monitoring for long-running sessions with resource management

### Commands Overview
- `check` - Verify connectivity and response status for targets
- `trace` - Detailed network timing analysis (DNS, connection, TLS, first byte)
- `monitor` - Continuous monitoring with data collection and alerting
- `analyze` - Statistical analysis of historical monitoring data
- `validate` - Configuration file validation and example generation
- `server` - Simple HTTP server for testing deployments

## Quick Start

### Installation

```shell
go install github.com/himasagaratluri/netirk
```

### Basic Usage

```shell
# Check connectivity to a single target
netirk check https://google.com

# Trace network timing for a target
netirk trace https://github.com

# Monitor multiple targets continuously
netirk monitor --targets-file targets.yaml --interval 30s

# Analyze historical monitoring data
netirk analyze --data-file monitoring.json --show-trends
```

## Enhanced Monitoring

### Continuous Monitoring

Monitor multiple targets with configurable intervals and comprehensive data collection:

```shell
# Basic monitoring with 30-second intervals
netirk monitor --targets-file targets.yaml --interval 30s

# Monitor for 1 hour and save results
netirk monitor --targets-file targets.yaml --duration 1h --output-file monitoring.json

# Enable alerts after 3 consecutive failures
netirk monitor --targets-file targets.yaml --alert-threshold 3 --alert-webhook https://hooks.slack.com/...

# Export results in multiple formats
netirk monitor --targets-file targets.yaml --output-file results --output-format html
```

### Configuration File Format

Create a `targets.yaml` file with enhanced monitoring configuration:

```yaml
targets:
  - url: "https://api.example.com"
    timeout: "30s"
    interval: "60s"
    expected_status: 200
    ssl_check: true
    headers:
      Authorization: "Bearer token"
      User-Agent: "Netirk Monitor"
  - url: "tcp://database.example.com:5432"
    timeout: "10s"
    interval: "30s"
  - url: "https://secure.example.com"
    ssl_check: true
    ssl_warning_days: 30
    ssl_critical_days: 7

# Alert configuration
alerts:
  enabled: true
  failure_threshold: 3
  webhook_url: "https://hooks.slack.com/services/..."
  email_recipients:
    - "admin@example.com"
  email_config:
    smtp_host: "smtp.gmail.com"
    smtp_port: 587
    username: "alerts@example.com"
    password: "app-password"
    use_starttls: true
```

### SSL Certificate Monitoring

```shell
# Check SSL certificate details
netirk check --target https://google.com --verify-ssl
```

Output:
```
Getting server certs...

➥ Cert: 0 
 ￫ CA: false
 ￫ Issuer: WR2
 ￫ Expiry: Monday, 13-Jan-25 08:36:56 UTC
 ￫ Days to expiry: 45
 ￫ Status: Valid
```

### Network Tracing

Detailed network timing analysis for troubleshooting performance issues:

```shell
# Trace network timing for HTTPS endpoint
netirk trace https://amazon.com
```

Output:
```
               _    _        _    
  _ __    ___ | |_ (_) _ __ | | __
 | '_ \  / _ \| __|| || '__|| |/ /
 | | | ||  __/| |_ | || |   |   < 
 |_| |_| \___| \__||_||_|   |_|\_\
                                  

DNS Resolution done: 7.553307ms
Connect Done: 27.578134ms
TLS Handshake Done: 83.745488ms
Time to first byte: 147.988115ms
Total Response Time: 266.865ms
```

### Historical Data Analysis

Analyze monitoring data with comprehensive statistics and trend detection:

```shell
# Basic analysis of monitoring data
netirk analyze --data-file monitoring.json

# Include trend analysis and anomaly detection
netirk analyze --data-file monitoring.json --show-trends

# Compare performance between time periods
netirk analyze --data-file monitoring.json --compare-periods

# Export analysis as JSON for integration
netirk analyze --data-file monitoring.json --output json
```

Sample analysis output:
```
=== Monitoring Session Analysis ===
Session Duration: 2h 15m 30s
Total Targets: 3
Total Checks: 270
Overall Success Rate: 98.52%

Target: https://api.example.com
  Total Checks: 90
  Success Rate: 100.00% (0 failures)
  Response Times:
    Average: 145ms
    Min: 89ms
    Max: 234ms
    95th percentile: 198ms
    99th percentile: 220ms
  Trends:
    response_time: stable [confidence: 0.85]
    success_rate: stable [confidence: 1.00]
```

## Export and Integration

### Multiple Output Formats

Export monitoring data in various formats for integration with other tools:

```shell
# Export as JSON (default)
netirk monitor --targets-file targets.yaml --output-file results.json

# Generate HTML report with charts
netirk monitor --targets-file targets.yaml --output-file report.html --output-format html

# Export CSV for spreadsheet analysis
netirk monitor --targets-file targets.yaml --output-file data.csv --output-format csv

# Generate Prometheus metrics
netirk monitor --targets-file targets.yaml --output-file metrics.prom --output-format prometheus
```

### Prometheus Integration

The Prometheus export format includes comprehensive metrics:

```
# Response time metrics
netirk_response_time_seconds{target="https://api.example.com",stat="current"} 0.145
netirk_response_time_seconds{target="https://api.example.com",stat="avg"} 0.152

# Success rate and uptime
netirk_success_rate{target="https://api.example.com"} 100.00
netirk_up{target="https://api.example.com"} 1

# SSL certificate metrics
netirk_ssl_cert_expiry_days{target="https://api.example.com",issuer="Let's Encrypt"} 45
netirk_ssl_cert_valid{target="https://api.example.com",issuer="Let's Encrypt"} 1

# Network timing breakdown
netirk_dns_duration_seconds{target="https://api.example.com"} 0.008
netirk_connect_duration_seconds{target="https://api.example.com"} 0.028
netirk_tls_duration_seconds{target="https://api.example.com"} 0.084
```

## Configuration and Validation

### Generate Example Configuration

```shell
# Generate a comprehensive example configuration
netirk validate --generate-example my-config.yaml

# Validate existing configuration
netirk validate targets.yaml
```

### Test Server

Start a simple HTTP server for testing monitoring setups:

```shell
# Start test server on default port 8080
netirk server

# Test the server endpoints
curl localhost:8080/health  # Returns: healthy
curl localhost:8080/host    # Returns: hostname
```

## Development

### Building from Source

```shell
# Clone the repository
git clone https://github.com/himasagaratluri/netirk.git
cd netirk

# Build the project
go build -o netirk

# Run tests
go test ./...

# Install locally
go install
```

### Using the Makefile

The project includes a comprehensive Makefile for development tasks:

```shell
# Build the project
make build

# Run all tests with coverage
make test-coverage

# Format code and run quality checks
make check

# Build for all platforms
make build-all

# Clean build artifacts
make clean

# See all available targets
make help
```

## Examples

Check the [examples](./examples/) directory for sample configuration files:

- `simple-monitoring.yaml` - Basic HTTP/HTTPS monitoring
- `advanced-monitoring.yaml` - Complex setup with alerts and SSL monitoring
- `ssl-monitoring.yaml` - SSL certificate focused monitoring
- `enhanced-targets.yaml` - Multiple targets with different configurations

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## License

This project is licensed under the terms specified in the LICENSE file.
