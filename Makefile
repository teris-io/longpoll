APP=$(shell basename `pwd`)

build:
	@echo ${APP}::clean
	@go clean -i ./...
	@echo ${APP}::format
	@go fmt ./...
	@echo ${APP}::get
	@go get
	@echo ${APP}::build
	@go build
	@echo ${APP}::test
	@go test -cover ./...
