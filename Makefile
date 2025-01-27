tidy:
	@go mod tidy
	@cd ./core && go mod tidy
	@cd ./cosmos && go mod tidy

unit-test:
	@docker pull nginx:latest
	@docker pull interchainio/simapp:latest
	@cd ./core && go test ./... -race
	@cd ./cosmos && go test `go list ./... | grep -v e2e` -race

docker-e2e:
	@docker pull nginx:latest
	@docker pull interchainio/simapp:latest
	@cd ./cosmos && go test ./tests/e2e/docker/... -race -v

digitalocean-e2e:
	@cd ./cosmos && go test ./tests/e2e/digitalocean/... -race -v

e2e-test: docker-e2e digitalocean-e2e

govulncheck:
	@echo "--> Running govulncheck"
	@go run golang.org/x/vuln/cmd/govulncheck -test ./...

lint:
	@echo "--> Running linter for core pkg..."
	@cd ./core && go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --out-format=tab
	@echo "--> Running linter for cosmos pkg..."
	@cd ./cosmos && go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --out-format=tab

lint-fix:
	@echo "--> Running linter for core pkg..."
	@cd ./core && go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --fix --out-format=tab --issues-exit-code=0
	@echo "--> Running linter for cosmos pkg..."
	@cd ./cosmos && go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --fix --out-format=tab --issues-exit-code=0

format:
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run mvdan.cc/gofumpt -w .
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run github.com/client9/misspell/cmd/misspell -w
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run golang.org/x/tools/cmd/goimports -w -local github.com/skip-mev/petri

.PHONY: format lint-fix lint govulncheck unit-test docker-e2e digitalocean-e2e e2e-test tidy
