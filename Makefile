APP=$(shell basename `pwd`)
LDFLAGS += -X "github.com/ventu-io/${APP}/settings.BuildDate=$(shell date +'%s')"

build: clean format test compile

clean:
	@echo ${APP}::clean
	@go clean -i ./...
	@rm -f ${GOPATH}/bin/${APP}

format:
	@echo ${APP}::format
	@go fmt ./...

test:
	@echo ${APP}::test
	@go test -v -ldflags '$(LDFLAGS)' ./...

compile:
	@echo ${APP}::build
	@go build -ldflags '$(LDFLAGS)'
