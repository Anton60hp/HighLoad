# Мультистейдж Dockerfile для оптимизации размера образа

# Этап 1: Сборка
FROM golang:1.22-alpine AS builder

# Установка toolchain для автоматического обновления Go при необходимости
ENV GOTOOLCHAIN=auto

# Установка зависимостей для сборки
RUN apk add --no-cache git ca-certificates

# Рабочая директория
WORKDIR /build

# Копирование go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка бинарника
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o iot-processor .

# Этап 2: Финальный образ
FROM alpine:latest

# Установка CA сертификатов для HTTPS запросов
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Копирование бинарника из builder
COPY --from=builder /build/iot-processor .

# Создание непривилегированного пользователя
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Порт приложения
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Запуск приложения
CMD ["./iot-processor"]

