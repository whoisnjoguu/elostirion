.PHONY: build-cli

build-cli:
	@echo "Building elo CLI..."
	go build -o tmp/bin/elo ./cmd/elo
	@echo "Building elo CLI done."
