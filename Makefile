tidy:
	@go mod tidy

test:
	@go test ./... -race