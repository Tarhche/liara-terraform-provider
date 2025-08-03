imageName = terraform-provider-liara
openApiSpecVersion = 031070d6e1ca4c6bc70ad06b490fc5a8f0398add
buildCommand = docker build --build-arg OPENAPI_SPEC_VERSION=$(openApiSpecVersion) --tag $(imageName) --file Dockerfile .

default: openapi-clean fmt lint install generate openapi-generate

build-docker-image:
	@if [ -z "$$(docker images --quiet $(imageName) 2>/dev/null)" ]; then \
		echo "Building Docker image..."; \
		$(buildCommand); \
	fi

rebuild-docker-image:
	$(buildCommand) --no-cache

build: build-docker-image
	docker run --rm --volume $(PWD):/home $(imageName) sh -c "go build -v ./..."

install: build
	docker run --rm --volume $(PWD):/home $(imageName) sh -c "go install -v ./..."

lint: build-docker-image
	docker run --rm --volume $(PWD):/home $(imageName) sh -c "go tool golangci-lint run"

generate: build-docker-image
	docker run --rm --volume $(PWD):/home $(imageName) sh -c "cd tools; go generate ./..."

fmt: build-docker-image
	docker run --rm --volume $(PWD):/home $(imageName) sh -c "gofmt -s -w -e ."

test: build-docker-image
	docker run --rm --volume $(PWD):/home $(imageName) sh -c "go test -v -cover -timeout=120s -parallel=10 ./..."

testacc: build-docker-image
	docker run --rm --volume $(PWD):/home $(imageName) sh -c "TF_ACC=1 go test -v -cover -timeout 120m ./..."

generate-openapi: build-docker-image
	docker run --rm --volume $(PWD):/home $(imageName) sh -c  "go generate ./openapi/openapi.go"

clean-openapi: build-docker-image
	docker run --rm --volume $(PWD):/home $(imageName) sh -c "rm -rf openapi/*/"

terminal: build-docker-image
	docker run --interactive --tty --rm --volume $(PWD):/home $(imageName) sh

.PHONY: fmt lint test testacc build install generate build-docker-image rebuild-docker-image generate-openapi clean-openapi terminal
