FROM golang:1.25-alpine AS build-stage

RUN apk add --no-cache git tzdata

WORKDIR /app

# Copy go mod and vendor directory
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# Copy source
COPY . .

RUN go build -mod=vendor -o cipher ./cmd/api

FROM alpine:latest

# poppler-utils provides pdftoppm, which renders statement PDFs into page
# images for the statement.extract job.
RUN apk update --no-cache && apk add --no-cache ca-certificates tzdata poppler-utils

WORKDIR /root/

COPY --from=build-stage /app/cipher .

ENV TZ=Asia/Kolkata

EXPOSE 5160

ENTRYPOINT ["./cipher"]
