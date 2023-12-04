lint:
	@echo "--> Running linter"
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run --out-format=tab

format:
	@echo "--> Running gofmt"
	@gofmt -s -w .
