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

.PHONY: fmt
fmt:
	gofmt -s -w .
	goimports -w .
