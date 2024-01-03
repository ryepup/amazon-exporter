# Stage 1: Build the Go application
FROM golang:latest AS builder

WORKDIR /app

COPY go.* ./
RUN go mod download

# Build the Go binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app

FROM alpine:latest as user

RUN addgroup -g 1000 appuser \
    && adduser -u 1000 -G appuser -SDH appuser

# Stage 2: Create a minimal Docker image
FROM scratch

COPY --from=user /etc/passwd /etc/passwd
USER appuser

# Copy the binary from the builder stage
COPY --from=builder /app/app /app/
WORKDIR /app

# Command to run the executable
ENTRYPOINT ["/app/app"]
