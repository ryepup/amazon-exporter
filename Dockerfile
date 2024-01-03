# Stage 1: Build the Go application
FROM golang:latest AS builder

WORKDIR /app

COPY . .

# Build the Go binary
RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux go build -o app

# Stage 2: Create a minimal Docker image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/app /app/

# Expose the port on which the application will run
EXPOSE 8080

# Set environment variables
ENV PORT=8080

# Command to run the executable
CMD ["/app/app", "-port", "8080"]
