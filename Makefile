.PHONY: test demo demo-konflux fmt

test:
	go test ./...

demo:
	go run . -provider github-checks examples/warning.json

demo-konflux:
	@echo 'Konflux TEST_OUTPUT:'
	@jq -r '.status.results[] | select(.name == "TEST_OUTPUT").value | fromjson' examples/konflux/taskrun-warning.json
	@printf '\nGitHub Checks decision:\n'
	@go run . -provider github-checks examples/konflux/pipelinerun-warning.json

fmt:
	gofmt -w main.go main_test.go
