# Build stage
FROM golang:1.26-alpine AS builder

# Install certs + timezone data so they can be copied into the scratch image
RUN apk --no-cache add ca-certificates tzdata

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies.
# Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go app statically with stripped symbols for a smaller binary size
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o server ./cmd/server

# Run stage
FROM scratch

# Copy CA certificates (for secure outgoing network calls) and timezone data
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Set the working directory
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/server .

# Run as non-root (nobody)
USER 65534:65534

# Expose port 8000 by default (respects the PORT env variable at runtime)
EXPOSE 8000

# Command to run the executable
CMD ["./server"]
