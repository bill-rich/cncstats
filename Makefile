.PHONY: test

test:
	go test -timeout=5m $(shell go list ./...)
