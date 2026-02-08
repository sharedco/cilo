# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o cilo-server ./cmd/server

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates wget

# Copy binary
COPY --from=builder /app/cilo-server .

# Copy migrations
COPY --from=builder /app/pkg/store/migrations ./migrations

# Create non-root user
RUN adduser -D -H cilo
USER cilo

EXPOSE 8080

ENTRYPOINT ["./cilo-server"]
