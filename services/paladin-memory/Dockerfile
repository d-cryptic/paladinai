# syntax=docker/dockerfile:1.7
# Multi-stage build for paladin-memory
FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /bin/paladin-memory ./services/paladin-memory/cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S paladin && adduser -S -G paladin paladin
COPY --from=builder /bin/paladin-memory /paladin-memory
USER paladin
EXPOSE 9010
ENTRYPOINT ["/paladin-memory"]
