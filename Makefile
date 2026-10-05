.PHONY: test demo fmt

test:
	go test ./...

demo:
	go run . -provider github-checks examples/warning.json

fmt:
	gofmt -w main.go main_test.go
