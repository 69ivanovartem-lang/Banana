# Этап 1: сборка
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY go.sum* ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /banana-server .

# Этап 2: лёгкий образ
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /banana-server ./banana-server
COPY --from=builder /src/index.html* ./
ENV BANANA_DATA_DIR=/app/data BANANA_ADDR=0.0.0.0 BANANA_PORT=8080
EXPOSE 8080
VOLUME ["/app/data"]
CMD ["./banana-server"]