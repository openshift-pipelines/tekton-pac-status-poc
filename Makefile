.PHONY: test demo demo-konflux fmt

test:
	go test ./...

demo:
	go run . -provider github-checks examples/warning.json

demo-konflux:
	@echo 'Observed public Konflux output:'
	@jq '{source, githubCheck, taskResult}' examples/konflux/observed-warning.json
	@printf '\nPoC GitHub Checks decision:\n'
	@go run . -provider github-checks examples/konflux/pipelinerun-warning.json

fmt:
	gofmt -w main.go main_test.go
