# Variables
BINARY_NAME=ppu-exporter
DOCKER_IMAGE=ppu-exporter-mock
DOCKER_TAG=latest
PORT=8080

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

.PHONY: all build clean test deps docker-build docker-run docker-stop help

# Default target
all: clean deps test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 GOOS=linux $(GOBUILD) -a -installsuffix cgo -o $(BINARY_NAME) main.go

# Build for current OS
build-local:
	@echo "Building $(BINARY_NAME) for local OS..."
	$(GOBUILD) -o $(BINARY_NAME) main.go

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Run the application locally
run: build-local
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME) --node-name=ppu-worker-local --port=$(PORT)

# Build Docker image
docker-build:
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Run Docker container
docker-run: docker-build
	@echo "Running Docker container..."
	docker run -d --name $(BINARY_NAME) -p $(PORT):8080 \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Run Docker container with custom node name
docker-run-custom: docker-build
	@echo "Running Docker container with custom settings..."
	docker run -d --name $(BINARY_NAME) -p $(PORT):8080 \
		$(DOCKER_IMAGE):$(DOCKER_TAG) \
		./ppu-exporter --node-name=ppu-worker-custom --gpu-count=8

# Stop and remove Docker container
docker-stop:
	@echo "Stopping Docker container..."
	-docker stop $(BINARY_NAME)
	-docker rm $(BINARY_NAME)

# View Docker logs
docker-logs:
	docker logs -f $(BINARY_NAME)

# Push Docker image (requires DOCKER_REGISTRY environment variable)
docker-push: docker-build
	@echo "Pushing Docker image..."
	@if [ -z "$(DOCKER_REGISTRY)" ]; then \
		echo "Error: DOCKER_REGISTRY environment variable not set"; \
		exit 1; \
	fi
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)

# Show available targets
help:
	@echo "Available targets:"
	@echo "  all          - Clean, download deps, test, and build"
	@echo "  build        - Build binary for Linux"
	@echo "  build-local  - Build binary for current OS"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  run          - Build and run locally"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Build and run Docker container"
	@echo "  docker-run-custom - Run with custom settings"
	@echo "  docker-stop  - Stop and remove Docker container"
	@echo "  docker-logs  - View Docker container logs"
	@echo "  docker-push  - Push Docker image (requires DOCKER_REGISTRY)"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  DOCKER_REGISTRY - Docker registry for pushing images"
	@echo "  PORT           - Port to run on (default: 8080)"