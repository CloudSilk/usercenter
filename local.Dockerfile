FROM guoxf/golang-run:alpine-3.13.5
LABEL MAINTAINER="ants.guoxf@gmail.com"

ENV DUBBO_GO_CONFIG_PATH="./dubbogo.yaml"

WORKDIR /workspace
COPY usercenter ./
COPY docs/swagger.json ./docs
COPY docs/swagger.yaml ./docs

EXPOSE 20000

ENTRYPOINT ./usercenter
