# Stage 1: build the go binary
FROM golang:1.23-alpine AS build-stage

RUN apk add --no-cache git tzdata

WORKDIR /app

# Copy go mod and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN go build -o api .

# Stage 2: Create a minimal image
FROM alpine:latest

# Add CA certificates (needed for HTTPS/gcloud/pubsub)
RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata

WORKDIR /root/

# Copy binary from build-stage
COPY --from=build-stage /app .

# Set timezone for logs and time library
ENV TZ=Asia/Kolkata

EXPOSE 5151

# Run the binary
ENTRYPOINT [ "./api" ]
