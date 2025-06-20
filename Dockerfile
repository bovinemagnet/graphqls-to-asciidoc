# Build stage
FROM golang:1.22-alpine AS builder

# Install git for version info
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o graphqls-to-asciidoc .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/graphqls-to-asciidoc .

# Create directory for schemas
RUN mkdir -p /schemas

# Set the binary as entrypoint
ENTRYPOINT ["./graphqls-to-asciidoc"]

# Default command
CMD ["-help"]