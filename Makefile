.PHONY: build test vet lint clean cross dist docker docker-run

BINARY := nanobot
MODULE := github.com/coopco/nanobot
LDFLAGS := -s -w
IMAGE := nanobot:latest

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/nanobot

test:
	go test -race -cover ./...

vet:
	go vet ./...

lint: vet
	@which staticcheck > /dev/null 2>&1 || echo "install: go install honnef.co/go/tools/cmd/staticcheck@latest"
	staticcheck ./... || true

clean:
	rm -f $(BINARY)
	rm -rf dist/

dist:
	mkdir -p dist

cross: dist
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64  ./cmd/nanobot
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64  ./cmd/nanobot
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 ./cmd/nanobot
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./cmd/nanobot

docker:
	docker build -t $(IMAGE) .

docker-run:
	docker run --rm $(IMAGE) agent -m "Hello"
