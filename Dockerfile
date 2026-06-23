# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git build-base

WORKDIR /app

COPY go.mod ./
# RUN go mod download

COPY . .

# Build static binary with optimized linker flags
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cromedia main.go

# Final light stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/cromedia .

ENTRYPOINT ["./cromedia"]
CMD ["help"]
