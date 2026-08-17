FROM golang:1.22-alpine AS builder

WORKDIR /app

# Enable CGO for go-sqlite3
ENV CGO_ENABLED=1
RUN apk add --no-cache gcc musl-dev

COPY . .
RUN go mod tidy
RUN go build -ldflags="-s -w" -o nexus-api main.go

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache tzdata ca-certificates

# Copy binary and frontend assets
COPY --from=builder /app/nexus-api .
COPY --from=builder /app/frontend ./frontend

# Create data directory for SQLite
RUN mkdir -p /app/data

EXPOSE 8080
CMD ["./nexus-api"]
