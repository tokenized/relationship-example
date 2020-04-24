BUILD_DATE = `date +%FT%T%z`
BUILD_USER = $(USER)@`hostname`
VERSION = `git describe --tags`

# command to build and run on the local OS.
GO_BUILD = go build

# command to compiling the distributable. Specify GOOS and GOARCH for the target OS.
GO_DIST = CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO_BUILD) -a -tags netgo -ldflags "-w -X main.buildVersion=$(VERSION) -X main.buildDate=$(BUILD_DATE) -X main.buildUser=$(BUILD_USER)"

BINARY=relationshipd
CLI_BINARY=relationship

.PHONY:

dist:
	$(GO_DIST) -o dist/$(BINARY) cmd/daemon/main.go
	$(GO_DIST) -o dist/$(CLI_BINARY) cmd/client/main.go

run-client:
	go run cmd/client/main.go

run-daemon:
	go run cmd/daemon/main.go

deps:
	go get -t ./...

test:
	go test ./...

test-all:
	go clean -testcache
	go test ./...

lint: golint vet goimports

vet:
	ret=0 && test -z "$$(go vet ./... | tee /dev/stderr)" || ret=1 ; exit $$ret

golint:
	ret=0 && test -z "$$(golint . | tee /dev/stderr)" || ret=1 ; exit $$ret

goimports:
	ret=0 && test -z "$$(goimports -l . | tee /dev/stderr)" || ret=1 ; exit $$ret

tools:
	[ -f $(GOPATH)/bin/goimports ] || go get golang.org/x/tools/cmd/goimports
	[ -f $(GOPATH)/bin/golint ] || go get github.com/golang/lint/golint
