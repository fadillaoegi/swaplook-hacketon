FROM node:24-alpine AS web
WORKDIR /web
RUN npm install --global pnpm@11.1.3
COPY swaplook_FE/package.json swaplook_FE/pnpm-lock.yaml swaplook_FE/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY swaplook_FE/ ./
RUN pnpm build

FROM golang:1.26-alpine AS api
WORKDIR /source
COPY swaplook_BE/go.mod swaplook_BE/go.sum ./
RUN go mod download
COPY swaplook_BE/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /swaplook ./cmd/api

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -S -g 10001 swaplook && adduser -S -D -H -u 10001 -G swaplook swaplook
WORKDIR /app
COPY --from=api /swaplook /app/swaplook
COPY --from=web --chown=10001:10001 /web/dist /app/public
RUN mkdir -p /app/data/images /app/data/garments && chown -R 10001:10001 /app/data
USER 10001:10001
ENV HTTP_ADDR=0.0.0.0:8088 STATIC_DIR=/app/public PRIVATE_STORAGE_DIR=/app/data/images YOUCAM_GARMENT_DIR=/app/data/garments
EXPOSE 8088
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD wget -q -O /dev/null http://127.0.0.1:8088/api/health || exit 1
ENTRYPOINT ["/app/swaplook"]
