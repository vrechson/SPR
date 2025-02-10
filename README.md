# SPR - Swagger Proxy Runner

SPR (Swagger Proxy Runner) automates API requests based on a Swagger file, using multithreading with a semaphore to limit concurrent requests, and routes them through a proxy for analysis.

## Features
- Reads API endpoints from a Swagger JSON file.
- Supports `GET` requests with query parameters.
- Uses `threads` and a `semaphore` (limit of 10 concurrent requests).
- Sends requests through a proxy (`Burp Suite` by default).

## Requirements
- Python 3.x
- `requests` module

## Installation
Clone this repository and install dependencies:
```sh
git clone https://github.com/arthur4ires/SPR
cd SPR
pip install -r requirements.txt
```

## Usage
```sh
python spr.py <swagger_file.json> <base_url>
```
Example:
```sh
python spr.py swagger.json https://example.com
```

## Configuration
- The script processes only `GET` requests for now.
- It replaces path parameters (`{}`) with a default value (`123`).
- Uses a proxy (`127.0.0.1:8080`) for traffic analysis.

## Contributing
Feel free to submit pull requests for enhancements or additional HTTP methods.

## License
MIT License.
