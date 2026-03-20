.PHONY: build clean dev web web-install run docker docker-omni docker-omni-nvidia docker-omni-amd

# Build everything
build: web
	go build -o lmgate .

# Build Go binary only (assumes web already built)
build-go:
	go build -o lmgate .

# Install web dependencies
web-install:
	cd web && npm install

# Build web frontend
web: web-install
	cd web && npm run build

# Development: run Go server with TLS disabled
dev:
	LMGATE_TLS_DISABLED=true go run .

# Clean build artifacts
clean:
	rm -f lmgate
	rm -rf web/build web/node_modules

# Docker build
docker:
	docker build -t lmgate .

# Docker build (omni CPU)
docker-omni:
	docker build -t lmgate:omni -f Dockerfile.omni .

# Docker build (omni NVIDIA)
docker-omni-nvidia:
	docker build -t lmgate:omni-nvidia -f Dockerfile.omni.nvidia .

# Docker build (omni AMD)
docker-omni-amd:
	docker build -t lmgate:omni-amd -f Dockerfile.omni.amd .

# Run tests
test:
	go test ./...
