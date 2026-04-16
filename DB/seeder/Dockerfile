# ──── Stage 1: Build ─────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install git (required by go mod for some dependencies)
RUN apk --no-cache add git ca-certificates

# Download dependencies first (better layer caching)
COPY go.mod go.sum* ./
RUN go mod download && go mod tidy

# Build binary
COPY *.go ./
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /seeder .

# ──── Stage 2: Runtime ────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary
COPY --from=builder /seeder /app/seeder

# Default schema path — can be overridden by compose volume mount
# Schema files mounted via docker-compose volume at /app/schema
CMD ["/app/seeder"]
