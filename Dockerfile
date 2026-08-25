# syntax=docker/dockerfile:1

FROM golang:1.26.5-alpine AS build

WORKDIR /src

# Cachear el módulo antes de copiar el resto del código.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
RUN go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed


# Aplica el esquema y siembra los datos de prueba una vez, y termina (exit 0).
# El servicio "app" del compose espera a que este contenedor termine con éxito no a que quede levantado.
FROM alpine:3.20 AS db-admin

RUN apk add --no-cache sqlite && \
    addgroup -S app && adduser -S -G app app && \
    mkdir -p /data && chown app:app /data

WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/seed /app/seed
COPY docker/db-entrypoint.sh /app/
RUN chmod +x /app/db-entrypoint.sh

ENV DB_PATH=/data/app.db
VOLUME ["/data"]

USER app

ENTRYPOINT ["/app/db-entrypoint.sh"]

FROM alpine:3.20 AS final

RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S -G app app && \
    mkdir -p /data && chown app:app /data

WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/seed /app/seed

ENV PORT=8080 \
    DB_PATH=/data/app.db

USER app
EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["/app/server"]
