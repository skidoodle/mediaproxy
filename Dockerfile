FROM golang:1.26-alpine AS builder
RUN apk add --no-cache build-base vips-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /mediaproxy .

FROM alpine:latest
RUN apk add --no-cache vips
RUN adduser -D -g '' appuser
USER appuser
COPY --from=builder /mediaproxy /mediaproxy
EXPOSE 8080
ENTRYPOINT ["/mediaproxy"]
