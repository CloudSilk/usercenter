IMAGE=CloudSilk/usercenter:v1.4.0
WEB_DIR=web/admin-ui

run:
	DUBBO_GO_CONFIG_PATH="./dubbogo.yaml" go run main.go
run-lift:
	DUBBO_GO_CONFIG_PATH="./dubbogo-lift.yaml" go run main.go

# 构建前端 React 应用 → web/admin-ui/dist/（go:embed 依赖）
web:
	cd $(WEB_DIR) && npm install && npm run build

# 本地开发：devserver（纯 HTTP，直连 MySQL，自动建表 + 播种管理员）
dev:
	NO_PROXY=localhost,127.0.0.1 UC_PORT=48180 go run ./cmd/devserver/

# 前端 HMR 开发（需另开终端跑 make dev）
dev-web:
	cd $(WEB_DIR) && npm run dev

build-image:
	CGO_ENABLED=0  GOOS=linux  GOARCH=amd64 go build -o usercenter main.go
	sudo docker build -f local.Dockerfile -t ${IMAGE} .
	rm usercenter
test-image:
	docker run -v `pwd`:/workspace/code --env DUBBO_GO_CONFIG_PATH="./code/dubbogo.yaml" --rm  ${IMAGE}
push-image:
	sudo docker push ${IMAGE}
gen-doc:
	swag init --parseDependency --parseInternal --parseDepth 2
test:
	go test ./...
