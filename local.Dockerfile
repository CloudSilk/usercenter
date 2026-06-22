# ---- Stage 1: 构建前端 React 应用 ----
FROM node:20-alpine AS web-builder
WORKDIR /web
COPY web/admin-ui/package*.json ./
RUN npm ci --omit=dev || npm install
COPY web/admin-ui/ ./
RUN npm run build
# 产物在 /web/dist/

# ---- Stage 2: 构建后端 Go 二进制(嵌入前端) ----
FROM golang:1.23-alpine AS go-builder
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# 用 Stage 1 的构建产物覆盖(确保最新)
COPY --from=web-builder /web/dist ./web/admin-ui/dist
RUN go build -o usercenter main.go

# ---- Stage 3: 运行时(精简镜像) ----
FROM alpine:3.19
LABEL MAINTAINER="ants.guoxf@gmail.com"
RUN apk add --no-cache ca-certificates tzdata
ENV DUBBO_GO_CONFIG_PATH="./dubbogo.yaml" TZ=Asia/Shanghai
WORKDIR /workspace
COPY --from=go-builder /app/usercenter ./
COPY docs/swagger.json ./docs/ 2>/dev/null || true
COPY docs/swagger.yaml ./docs/ 2>/dev/null || true
EXPOSE 48080 20000
ENTRYPOINT ["./usercenter"]
