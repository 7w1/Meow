FROM golang:1.26.3-alpine3.23 AS builder
WORKDIR /app
COPY go.mod go.sum ./
COPY main.go vocalization.go multilingual.go ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o maas .

FROM scratch
COPY --from=builder /app/maas /maas
COPY config.json /config.json

EXPOSE 8000
ENTRYPOINT ["/maas"]
