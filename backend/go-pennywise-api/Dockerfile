# Stage 1: build the go binary
FROM golang:1.24-alpine AS build-stage

RUN apk add --no-cache git tzdata

WORKDIR /app

# Copy shared module (required by replace directive in go.mod)
COPY shared/ ./shared/

# Copy go mod and download dependencies
COPY go-pennywise-api/go.mod go-pennywise-api/go.sum ./go-pennywise-api/
WORKDIR /app/go-pennywise-api
RUN go mod download

# Copy source
COPY go-pennywise-api/ /app/go-pennywise-api/

# Build
RUN go build -o api ./cmd/api

# Stage 2: Create a minimal image
FROM alpine:latest

# Add CA certificates (needed for HTTPS/gcloud/pubsub)
RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata

WORKDIR /root/

# Copy binary from build-stage
COPY --from=build-stage /app/go-pennywise-api/api .

# Set timezone for logs and time library
ENV TZ=Asia/Kolkata

EXPOSE 5151

# Run the binary
ENTRYPOINT ["./api"]
