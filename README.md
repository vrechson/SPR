# SPR - Swagger Proxy Runner

SPR (Swagger Proxy Runner) automates API requests based on OpenAPI/Swagger specifications, using concurrent requests through a proxy for security testing and analysis.

## Features
- Reads API endpoints from OpenAPI/Swagger JSON files
- Supports multiple HTTP methods (GET, POST, PUT, DELETE, PATCH)
- Concurrent request execution with configurable thread count
- Proxy support for traffic analysis (e.g. Burp Suite)
- Integer parameter fuzzing with multiple test values
- Custom header support
- Parameter value overrides
- Progress bar for request tracking
- Verbose output mode

## Requirements
- Go 1.24+

## Installation
```bash
go install github.com/vrechson/spr@latest
```

## Examples

```
spr -swagger examples/swagger.json -host 'https://api.ganjoor.net/' -param-override="id=1337" --int-fuzzing -threads 50
```