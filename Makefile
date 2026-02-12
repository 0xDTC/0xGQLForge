.PHONY: build run clean

BINARY=gqlforge
MAIN=./cmd/gqlforge

build:
	CGO_ENABLED=1 go build -o $(BINARY) $(MAIN)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

tidy:
	go mod tidy
