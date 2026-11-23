
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Копируем файлы модуля и скачиваем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код и собираем бинарник
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/banana-backend

# Этап 2: Запуск
FROM alpine:latest
WORKDIR /app

# Копируем бинарник из этапа сборки
COPY --from=builder /app/banana-backend .

# Открываем порт
EXPOSE 8080

# Запускаем сервер
CMD ["./banana-backend"]