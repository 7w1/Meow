FROM golang:1.26.3-alpine3.23 AS builder
WORKDIR /app
COPY main.go .

# Compile a statically linked binary (CGO_ENABLED=0 ensures no external C libraries are needed)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o maas main.go

FROM scratch
COPY --from=builder /app/maas /maas
COPY config.json /config.json

EXPOSE 8000
ENTRYPOINT ["/maas"]