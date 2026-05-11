# syntax=docker/dockerfile:1.7
# Multi-stage build for paladin-edge
FROM golang:1.26-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /bin/paladin-edge ./services/paladin-edge/cmd/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S paladin && adduser -S -G paladin paladin
COPY --from=builder /bin/paladin-edge /paladin-edge
USER paladin
EXPOSE 8080
ENTRYPOINT ["/paladin-edge"]
