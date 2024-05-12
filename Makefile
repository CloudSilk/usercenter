IMAGE=CloudSilk/usercenter:v1.4.0
run:
	DUBBO_GO_CONFIG_PATH="./dubbogo.yaml" go run main.go
run-lift:
	DUBBO_GO_CONFIG_PATH="./dubbogo-lift.yaml" go run main.go
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