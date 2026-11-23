# Этап 1: Сборка фронтенда
FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# Этап 2: Запуск сервера из server.mjs
FROM node:20-alpine
WORKDIR /app
COPY --from=build /app/dist ./dist
COPY server.mjs ./
EXPOSE 80
CMD ["node", "server.mjs"]