APP=$(shell basename `pwd`)

build:
	@echo ${APP}::clean
	@go clean -i ./...
	@echo ${APP}::format
	@go fmt ./...
	@echo ${APP}::test
	@go test ./...
	@echo ${APP}::build lpoll
	@go build ./lpoll
