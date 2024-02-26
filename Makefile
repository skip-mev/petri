tidy:
	@go mod tidy
	@cd ./core && go mod tidy
	@cd ./cosmos && go mod tidy

test:
	@cd ./core && go test ./... -race
	@cd ./cosmos && go test ./... -race

govulncheck:
	@echo "--> Running govulncheck"
	@go run golang.org/x/vuln/cmd/govulncheck -test ./...

lint:
	@echo "--> Running linter"
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run core/util --out-format=tab

lint-fix:
	@echo "--> Running linter"
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run core cosmos/... --fix --out-format=tab --issues-exit-code=0

format:
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run mvdan.cc/gofumpt -w .
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run github.com/client9/misspell/cmd/misspell -w
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run golang.org/x/tools/cmd/goimports -w -local github.com/skip-mev/petri

.PHONY: format lint-fix lint govulncheck test tidy