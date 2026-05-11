# syntax=docker/dockerfile:1.7
# Multi-stage build for paladin-ingest
FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /bin/paladin-ingest ./services/paladin-ingest/cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S paladin && adduser -S -G paladin paladin
COPY --from=builder /bin/paladin-ingest /paladin-ingest
USER paladin
EXPOSE 9001
ENTRYPOINT ["/paladin-ingest"]
