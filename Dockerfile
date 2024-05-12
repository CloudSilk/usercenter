FROM registry.cn-shanghai.aliyuncs.com/swtsoft/golang-build:1.20.0-alpine3.17 as builder

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /workspace
COPY . .
ARG ARG_GIT_USERNAME
ARG ARG_GIT_PASSWORD
RUN git config --global url."https://${ARG_GIT_USERNAME}:${ARG_GIT_PASSWORD}@codeup.aliyun.com".insteadOf "https://codeup.aliyun.com"
RUN go env && go build -o usercenter main.go

FROM registry.cn-shanghai.aliyuncs.com/swtsoft/golang-run:alpine-3.16.0
LABEL MAINTAINER="ants.guoxf@gmail.com"

ENV DUBBO_GO_CONFIG_PATH="./dubbogo.yaml"

WORKDIR /workspace
COPY --from=builder /workspace/usercenter ./
COPY --from=builder /workspace/docs/swagger.json ./docs
COPY --from=builder /workspace/docs/swagger.yaml ./docs

EXPOSE 20000

ENTRYPOINT ./usercenter
