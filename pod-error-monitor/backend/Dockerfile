FROM golang:1.21-alpine

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Use a minimal alpine image
FROM alpine:latest

WORKDIR /app

# Copy the binary from builder
COPY --from=0 /app/main .

# Copy the default config file
COPY config.yaml /app/config.yaml

# Create directory for kubeconfig
RUN mkdir -p /root/.kube

# Add entrypoint script
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

# Create config directory
RUN mkdir -p /app/config

# Environment variable for config path
ENV POD_ERROR_MONITOR_CONFIG=/app/config.yaml

ENTRYPOINT ["./entrypoint.sh"] 