FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /api ./cmd/api

FROM alpine:3.23
RUN apk add --no-cache ca-certificates openssl && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /api /app/api
COPY frontend/ /app/frontend/
COPY docker-entrypoint.sh /app/entrypoint.sh
USER app
ENTRYPOINT ["/bin/sh", "/app/entrypoint.sh"]
