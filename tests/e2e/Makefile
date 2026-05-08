.PHONY: test test-fast

test:
	go test -v -timeout 10m ./...

test-fast:
	go test -v -timeout 10m -run TestE2E_ScanPipeline ./...
