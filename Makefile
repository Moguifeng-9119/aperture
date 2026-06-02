.PHONY: build run test lint clean dev

BINARY := aperture
CMD_DIR := .

build:
	go build -ldflags="-s -w" -o $(BINARY) $(CMD_DIR)

run: build
	./$(BINARY)

dev:
	go run $(CMD_DIR)

test:
	go test -race -cover ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...

clean:
	rm -f $(BINARY) coverage.out coverage.html

docker-build:
	docker build -t aperture:latest .

docker-run:
	docker run --rm -p 8080:8080 -v $(PWD)/config.yaml:/app/config.yaml aperture:latest
