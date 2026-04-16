FROM golang:1.24-alpine AS build-stage

RUN apk add --no-cache git tzdata

WORKDIR /app

# Copy go mod and vendor directory
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy source
COPY . .

RUN go build -mod=vendor -o orchestrator ./cmd/api

FROM alpine:latest

RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata

WORKDIR /root/

COPY --from=build-stage /app/orchestrator .

ENV TZ=Asia/Kolkata

EXPOSE 5160

ENTRYPOINT ["./orchestrator"]
