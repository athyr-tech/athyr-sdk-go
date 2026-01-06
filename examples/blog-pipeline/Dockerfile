# Multi-stage Dockerfile for blog pipeline agents
#
# Build any agent by specifying the AGENT build arg:
#   docker build --build-arg AGENT=outline-agent -t outline-agent .
#   docker build --build-arg AGENT=draft-agent -t draft-agent .
#   docker build --build-arg AGENT=edit-agent -t edit-agent .
#   docker build --build-arg AGENT=seo-agent -t seo-agent .
#   docker build --build-arg AGENT=orchestrator -t orchestrator .

# Build stage
FROM golang:1.24-alpine AS builder

ARG AGENT
RUN test -n "$AGENT" || (echo "AGENT build arg is required" && exit 1)

WORKDIR /build

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the specified agent
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" \
    -o /agent \
    ./examples/blog-pipeline/cmd/${AGENT}

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /agent /app/agent

ENTRYPOINT ["/app/agent"]
