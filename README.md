# MaaS (Meowing as a Service)

A low-latency Go API for generating and detecting feline vocalizations. It runs as a statically linked binary in a small Docker container.

Website: [meow.plz.pet](https://meow.plz.pet)

## API Endpoints

### 1. Generate

`GET /meow`

Generates a vocalization using procedural phonetic combinations.

**Example response:**

```json
{
  "generation_time": "14.625µs",
  "meow": "mreeeeooowww",
  "family": "meow",
  "language": "en"
}
```

### 2. Detect

`GET /ismeow?text={input}`

Checks whether the entire input is a meow, including stretched spellings like `meeeeooowowwwwww`.

**Example response:**

```json
{
  "detection_time": "18.210µs",
  "input": "mrrp",
  "is_meow": true,
  "meow_percentage": "100.0%",
  "squeezed_form": "mrrp",
  "family": "prrr",
  "match_type": "exact",
  "language": "en"
}
```

### 3. Detect meow-like text

`GET /meowlike?text={input}`

Recognizes meows in short messages, including `meows`, `meowing`, and `purrfect`. `/ismeow` remains the strict check.

### 4. Meow 8-ball

`GET /askmeow?text={question}`

Returns a deterministic answer in English.

### Languages

Add `lang=fr` (or another supported tag) to `/meow`, `/ismeow`, or `/meowlike`. These endpoints default to English. Detection also accepts `lang=auto` to find the language. `GET /languages` lists the supported tags.

## Configuration

The API reads `config.json` from the root directory. If missing, it uses default values. For example:

```json
{
  "port": "8000",
  "generate_endpoint": "/meow",
  "detect_endpoint": "/ismeow"
}
```

## Local Development

Requires Go 1.26.3 or later.

```bash
go run .
go test ./...
```
