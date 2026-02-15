.PHONY: build run test clean fmt vet lint tidy all docker-build docker-run docker-clean docker-push docker-ghcr-build docker-ghcr-push help

BINARY_NAME=vault-search
IMAGE_NAME=vault-search
IMAGE_TAG?=latest
GHCR_REGISTRY?=ghcr.io
GHCR_REPO?=$(GHCR_REGISTRY)/$(USER)/$(IMAGE_NAME)

build:
	go build -o $(BINARY_NAME) .

run:
	go run .

test:
	go test -v ./...

test-coverage:
	go test -cover ./...

clean:
	rm -f $(BINARY_NAME)

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	gosec ./...

tidy:
	go mod tidy

docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-run:
	docker run --rm -p 8080:8080 \
		-e VAULT_TOKEN=$(VAULT_TOKEN) \
		-e VAULT_ADDR=$(VAULT_ADDR) \
		-e VAULT_MOUNT_POINT=$(VAULT_MOUNT_POINT) \
		-e LOG_LEVEL=$(LOG_LEVEL) \
		$(IMAGE_NAME):$(IMAGE_TAG)

docker-clean:
	docker rmi $(IMAGE_NAME):$(IMAGE_TAG) 2>/dev/null || true

docker-push: docker-build
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

docker-ghcr-build:
	docker build -t $(GHCR_REPO):$(IMAGE_TAG) .

docker-ghcr-push: docker-ghcr-build
	docker push $(GHCR_REPO):$(IMAGE_TAG)

all: tidy fmt vet test build

help:
	@echo "Available commands:"
	@echo ""
	@echo "Build & Run:"
	@echo "  make build           - Build the binary"
	@echo "  make run             - Run the application"
	@echo "  make clean           - Remove built binary"
	@echo ""
	@echo "Testing:"
	@echo "  make test            - Run tests with verbose output"
	@echo "  make test-coverage   - Run tests with coverage"
	@echo ""
	@echo "Code Quality:"
	@echo "  make fmt             - Format code"
	@echo "  make vet             - Run go vet"
	@echo "  make lint            - Run gosec security scanner"
	@echo "  make tidy            - Run go mod tidy"
	@echo "  make all             - Run tidy, fmt, vet, test, and build"
	@echo ""
	@echo "Docker (local):"
	@echo "  make docker-build    - Build Docker image"
	@echo "  make docker-run      - Run Docker container"
	@echo "  make docker-clean    - Remove Docker image"
	@echo "  make docker-push     - Push to Docker registry"
	@echo ""
	@echo "Docker (GitHub Container Registry):"
	@echo "  make docker-ghcr-build - Build for ghcr.io"
	@echo "  make docker-ghcr-push  - Push to ghcr.io"
	@echo ""
	@echo "Variables:"
	@echo "  IMAGE_TAG=latest     - Docker image tag (default: latest)"
	@echo "  GHCR_REPO=ghcr.io/user/vault-search - GHCR repository"
	@echo "  VAULT_TOKEN=xxx      - Vault token (required)"
	@echo "  VAULT_ADDR=https://... - Vault address"
