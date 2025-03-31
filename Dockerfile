FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY . .

RUN apk add --no-cache openssl
RUN mkdir -p /app/certs && \
    chmod 700 /app/certs

COPY certs/gen_ca.sh certs/gen_cert.sh /app/certs/
RUN chmod +x /app/certs/gen_*.sh && \
    cd /app/certs && \
    ./gen_ca.sh

RUN go mod download
RUN go build -o proxy-scanner .

FROM alpine:latest
WORKDIR /app

COPY --from=builder /app/proxy-scanner .
COPY --from=builder /app/certs/ ./certs/

RUN mkdir -p /app/certs/certs_cache && \
    chmod -R 700 /app/certs && \
    chmod 600 /app/certs/ca.key && \
    chmod 644 /app/certs/ca.crt

EXPOSE 8080 8000
CMD ["./proxy-scanner"]