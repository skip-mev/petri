tidy:
	@go mod tidy

<<<<<<< HEAD
test:
	@go test ./... -race
=======
unit-test:
	@docker pull nginx:latest
	@docker pull ghcr.io/cosmos/simapp:v0.47
	@cd ./core && go test -v -count 2 ./... -race
	@cd ./cosmos && go test -v -count 2 `go list ./... | grep -v e2e` -race

docker-e2e:
	@docker pull nginx:latest
	@docker pull ghcr.io/cosmos/simapp:v0.47
	@cd ./cosmos && go test -v -count 1 ./tests/e2e/docker/... -race -v

digitalocean-e2e:
	@cd ./cosmos && go test -v -count 1 ./tests/e2e/digitalocean/... -race -v

e2e-test: docker-e2e digitalocean-e2e
>>>>>>> 54795a0 (fix: naming)

govulncheck:
	@echo "--> Running govulncheck"
	@go run golang.org/x/vuln/cmd/govulncheck -test ./...

lint:
	@echo "--> Running linter"
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run --out-format=tab
lint-fix:
	@echo "--> Running linter"
	@go run github.com/golangci/golangci-lint/cmd/golangci-lint run --fix --out-format=tab --issues-exit-code=0

format:
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run mvdan.cc/gofumpt -w .
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run github.com/client9/misspell/cmd/misspell -w
	@find . -name '*.go' -type f -not -path "*.git*" | xargs go run golang.org/x/tools/cmd/goimports -w -local github.com/skip-mev/petri

.PHONY: format lint-fix lint govulncheck test tidy