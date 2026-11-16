FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
RUN if [ ! -f go.mod ]; then go mod init banana; fi && go mod tidy
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /banana .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /banana /app/banana
ENV BANANA_ADDR=0.0.0.0 \
    BANANA_PORT=8080 \
    BANANA_DATA_DIR=/data
EXPOSE 8080
CMD ["/app/banana"]