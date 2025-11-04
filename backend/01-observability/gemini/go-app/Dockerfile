# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary
# -ldflags="-w -s" strips debug symbols to reduce binary size
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-w -s" -o go-app .

# Stage 2: Run
FROM alpine:3.18

WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/go-app .

# Expose the application port and the metrics port
EXPOSE 8080
EXPOSE 8081

# Run the binary
CMD ["./go-app"]
