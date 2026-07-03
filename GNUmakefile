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

.PHONY: release-check
release-check:
	goreleaser check

.PHONY: release-snapshot
release-snapshot:
	goreleaser release --snapshot --clean --skip=sign,publish

.PHONY: fmt
fmt:
	gofmt -s -w .
	goimports -w .
