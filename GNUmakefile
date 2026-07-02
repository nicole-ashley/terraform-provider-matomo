default: test

.PHONY: build
build:
	go build -o /dev/null .

.PHONY: test
test:
	go test ./... -v -count=1

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: docs
docs:
	go tool tfplugindocs generate

.PHONY: fmt
fmt:
	gofmt -s -w .
	goimports -w .
