APP=$(shell basename `pwd`)

build:
	@echo ${APP}::clean
	@go clean -i ./...
	@echo ${APP}::format
	@go fmt ./...
	@echo ${APP}::test
	@go test -cover -tags test ./...
	@echo ${APP}::build longpoll
	@go build ./longpoll
