# Build stage
FROM golang:1.26-alpine AS builder

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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o server ./cmd/server

# Run stage
FROM alpine:latest

# Install ca-certificates (for secure outgoing network calls) and tzdata (for timezone handling)
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user for running the application securely
RUN adduser -D -u 1000 appuser
USER appuser

# Set the working directory
WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/server .

# Expose port 8000 by default (respects the PORT env variable at runtime)
EXPOSE 8000

# Command to run the executable
CMD ["./server"]
