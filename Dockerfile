# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build-api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /org-cost-api ./cmd/server

FROM node:20-alpine AS build-frontend
ARG VITE_CHAT_URL=
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
ENV VITE_CHAT_URL=$VITE_CHAT_URL
RUN npm run build

FROM alpine:3.20 AS demo
RUN apk add --no-cache ca-certificates curl \
	&& adduser -D -H -u 10001 app \
	&& mkdir -p /data/history /app/static /config \
	&& chown -R app:app /data /app /config
WORKDIR /app
COPY --from=build-api --chown=app:app /org-cost-api /app/org-cost-api
COPY --from=build-frontend --chown=app:app /frontend/dist /app/static
COPY --chown=app:app config.demo.yaml /config/demo.yaml
USER app
EXPOSE 8080
ENV ORG_COST_DEMO=1
ENV CONFIG_PATH=/config/demo.yaml
ENV STATIC_DIR=/app/static
ENV HISTORY_DIR=/data/history
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
	CMD curl -fsS http://127.0.0.1:8080/api/ready >/dev/null || exit 1
ENTRYPOINT ["/app/org-cost-api", "-config", "/config/demo.yaml"]

FROM alpine:3.20 AS prod-ui
RUN apk add --no-cache ca-certificates curl \
	&& adduser -D -H -u 10001 app \
	&& mkdir -p /data/history /app/static /config \
	&& chown -R app:app /data /app /config
WORKDIR /app
COPY --from=build-api --chown=app:app /org-cost-api /app/org-cost-api
COPY --from=build-frontend --chown=app:app /frontend/dist /app/static
USER app
EXPOSE 8080
ENV CONFIG_PATH=/config/config.yaml
ENV STATIC_DIR=/app/static
ENV HISTORY_DIR=/data/history
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
	CMD curl -fsS http://127.0.0.1:8080/api/ready >/dev/null || exit 1
ENTRYPOINT ["/app/org-cost-api", "-config", "/config/config.yaml"]

FROM alpine:3.20 AS api
RUN apk add --no-cache ca-certificates curl \
	&& adduser -D -H -u 10001 app \
	&& mkdir -p /data/history /app \
	&& chown -R app:app /data /app
WORKDIR /app
COPY --from=build-api --chown=app:app /org-cost-api /app/org-cost-api
USER app
EXPOSE 8080
ENV CONFIG_PATH=/config/config.yaml
ENV HISTORY_DIR=/data/history
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
	CMD curl -fsS http://127.0.0.1:8080/api/ready >/dev/null || exit 1
ENTRYPOINT ["/app/org-cost-api", "-config", "/config/config.yaml"]
