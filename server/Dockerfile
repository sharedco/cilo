FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o cilo-server ./cmd/cilo-server

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/cilo-server .
COPY --from=builder /app/internal/server/store/migrations /app/pkg/store/migrations

EXPOSE 8080

CMD ["./cilo-server", "serve"]
