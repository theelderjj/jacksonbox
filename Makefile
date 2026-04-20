.PHONY: verify test-go test-client

verify: test-go test-client

test-go:
	go test ./cmd/... ./internal/...

test-client:
	cd client && npm run typecheck && npm test && npm run build
