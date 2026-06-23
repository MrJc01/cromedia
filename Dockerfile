# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git build-base ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build statically linked binary with legacy codecs
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -tags "legacy legacy_avi legacy_asf legacy_rm legacy_mp2 legacy_codecs" \
    -o cromedia main.go

# Final clean runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates ffmpeg

WORKDIR /app

COPY --from=builder /app/cromedia /app/cromedia

# Set environment variables for plugin path and strict checks
ENV CROMEDIA_PLUGINS_PATH="/app/plugins"
ENV CROMEDIA_STRICT="0"

ENTRYPOINT ["/app/cromedia"]
CMD ["help"]
