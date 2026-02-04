# Dockerfile for kvlite

# Build stage
FROM golang:1.25.6-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kvlite ./cmd/kvlite/

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/kvlite .

# Expose port
EXPOSE 6380

# Create non-root user
RUN addgroup -S kvlite && adduser -S kvlite -G kvlite
USER kvlite

# Run the binary
ENTRYPOINT ["./kvlite"]
CMD ["-host", "0.0.0.0"]