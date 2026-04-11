FROM golang:1.24-alpine AS build-stage

RUN apk add --no-cache git tzdata

WORKDIR /app

# Copy shared module first
COPY ../shared /shared

# Copy orchestrator module
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o orchestrator ./cmd/api

FROM alpine:latest

RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata

WORKDIR /root/

COPY --from=build-stage /app/orchestrator .

ENV TZ=Asia/Kolkata

EXPOSE 5160

ENTRYPOINT ["./orchestrator"]
