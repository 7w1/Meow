# MaaS (Meowing as a Service)

A low-latency API built in Go for generating and detecting feline vocalizations. The application is designed for high throughput and minimal system footprint, deployed as a statically linked binary within a scratch Docker container.

## API Endpoints

### 1. Generate 
`GET /meow`
Generates a vocalization using procedural phonetic combinations.

**Response:**
```json
{
  "generation_time": "14.625µs",
  "meow": "mreeeeooowww"
}
```

### 2. Detect

`GET /ismeow?text={input}`
Analyzes an input string against phonetic trait parameters to calculate a confidence score.

**Response:**

```json
{
  "detection_time": "18.210µs",
  "input": "mruuuuuurp",
  "is_meow": true,
  "meow_percentage": "100.0%",
  "squeezed_form": "mrup"
}
```

## Configuration

The application reads from `config.json` in the root directory. If missing, it uses default values.

```json
{
    "port": "8000",
    "generate_endpoint": "/meow",
    "detect_endpoint": "/ismeow"
}
```

## Local Development

### Requirements

* Go 1.26.3

### Build and Run

```bash
git clone https://github.com/7w1/Meow.git
cd maas
go run main.go
```

### Testing

Run the 100,000-iteration test suite:

```bash
go test -v
```