
# Step 1: Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the main application
RUN CGO_ENABLED=0 GOOS=linux go build -o /go-crud-app ./cmd/server

# Build the migration tool
RUN CGO_ENABLED=0 GOOS=linux go build -o /go-migrate-tool ./cmd/migrate

# Step 2: Final stage
FROM alpine:3.18

WORKDIR /app

# Copy the application binary from the builder stage
COPY --from=builder /go-crud-app .
# Copy the migration tool from the builder stage
COPY --from=builder /go-migrate-tool ./cmd/migrate

# Expose port 8080 for the server and 9091 for metrics
EXPOSE 8080
EXPOSE 9091

# Command to run the application
ENTRYPOINT ["/app/go-crud-app"]
