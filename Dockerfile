# Stage 1: Build SvelteKit frontend
FROM node:22-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS go-builder
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/build ./web/build
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o lmgate .

# Stage 3: Runtime
FROM alpine:3.19

LABEL org.opencontainers.image.authors="hkdb <hkdb@3df.io>"
LABEL org.opencontainers.image.source="https://github.com/hkdb/lmgate"
LABEL org.opencontainers.image.title="LM Gate Standalone Image"
LABEL org.opencontainers.image.licenses="Apache-2.0"
RUN apk add --no-cache ca-certificates tzdata su-exec && \
    addgroup -S lmgate && adduser -S lmgate -G lmgate
WORKDIR /app
COPY --from=go-builder /app/lmgate .
COPY config.yaml .
RUN mkdir -p /app/data && chown lmgate:lmgate /app/data

# Copy entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

VOLUME ["/app/data"]
EXPOSE 443 80 8080

ENTRYPOINT ["/entrypoint.sh"]
