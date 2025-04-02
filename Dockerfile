FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY . .

# Устанавливаем openssl и зависимости
RUN apk add --no-cache openssl bash make

# Создаем папку certs
RUN mkdir -p /app/certs && \
    chmod 700 /app/certs

# Копируем скрипты
COPY certs/gen_ca.sh certs/gen_cert.sh /app/certs/
RUN chmod +x /app/certs/*.sh

# Генерируем CA
RUN cd /app/certs && \
    ./gen_ca.sh

# Собираем приложение
RUN go mod download && \
    go build -o proxy-scanner .

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Устанавливаем openssl в финальный образ
RUN apk add --no-cache openssl bash

# Копируем бинарник и сертификаты
COPY --from=builder /app/proxy-scanner .
COPY --from=builder /app/certs /app/certs

# Настраиваем права
RUN mkdir -p /app/certs/certs_cache && \
    chmod -R 700 /app/certs && \
    chmod 600 /app/certs/ca.key && \
    chmod 644 /app/certs/ca.crt

EXPOSE 8080 8000
CMD ["./proxy-scanner"]