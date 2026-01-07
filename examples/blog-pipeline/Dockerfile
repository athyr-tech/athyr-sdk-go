# Multi-stage build for blog-pipeline agents
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy go mod files from repo root
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY api/ api/
COPY pkg/ pkg/
COPY examples/blog-pipeline/ examples/blog-pipeline/

# Build the specified agent
ARG AGENT=orchestrator
RUN CGO_ENABLED=0 go build -o /agent ./examples/blog-pipeline/cmd/${AGENT}

# Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /agent /usr/local/bin/agent
ENTRYPOINT ["/usr/local/bin/agent"]