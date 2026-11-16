# ---------- сборка фронта ----------
FROM node:20-alpine AS web
WORKDIR /web
COPY package*.json ./
RUN npm install --no-audit --no-fund
COPY index.html vite.config.js ./
COPY src ./src
RUN npm run build

# ---------- сборка бэка ----------
FROM golang:1.24-alpine AS api
WORKDIR /src
COPY *.go ./
RUN go mod init banana && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /banana .

# ---------- итог ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=api /banana /app/banana
COPY --from=web /web/dist /app/web
ENV BANANA_ADDR=0.0.0.0 \
    BANANA_PORT=8080 \
    BANANA_DATA_DIR=/data \
    BANANA_WEB_DIR=/app/web
VOLUME /data
EXPOSE 8080
CMD ["/app/banana"]