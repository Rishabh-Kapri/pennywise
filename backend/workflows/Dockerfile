# Stage 1: build the go binary
FROM golang:1.25-alpine AS build-stage

RUN apk add --no-cache git tzdata

WORKDIR /app

# Copy go mod and download dependencies
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy source
COPY . .

# Build
RUN go build -mod=vendor -o worker ./cmd/worker

# Stage 2: Create a minimal image
FROM alpine:latest

# Add CA certificates (needed for HTTPS/gcloud/pubsub)
RUN apk update --no-cache && apk add --no-cache ca-certificates

WORKDIR /root/

# Copy binary from build-stage
COPY --from=build-stage /app/worker .
  
# Set timezone for logs and time library
ENV TZ=Asia/Kolkata

EXPOSE 5001

# Run the binary
ENTRYPOINT ["./worker"]
