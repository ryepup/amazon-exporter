# Stage 1: Build the Go application
FROM golang:latest AS builder

WORKDIR /app

COPY go.* ./
RUN go mod download

# Build the Go binary
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o app

# Stage 2: Create a minimal Docker image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/app /app/

# Command to run the executable
ENTRYPOINT ["/app/app"]
