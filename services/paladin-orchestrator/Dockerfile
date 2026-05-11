# syntax=docker/dockerfile:1.7
# Multi-stage build for paladin-orchestrator
FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /bin/paladin-orchestrator ./services/paladin-orchestrator/cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S paladin && adduser -S -G paladin paladin
COPY --from=builder /bin/paladin-orchestrator /paladin-orchestrator
USER paladin
EXPOSE 9005
ENTRYPOINT ["/paladin-orchestrator"]
