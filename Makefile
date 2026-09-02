.PHONY: tidy
tidy:
	@echo 'Formatting code...'
	go fmt ./...
	@echo 'Tidying module dependencies...'
	go mod tidy -v

.PHONY: audit
audit:
	@echo 'Checking module dependencies...'
	go mod tidy -diff
	go mod verify
	@echo 'Checking formatting...'
	test -z "$(shell gofmt -l .)"
	@echo 'Vetting code...'
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...
	@echo 'Running tests...'
	go test -race -vet=off -count=1 ./...
	@echo 'Building application...'
	go build ./...