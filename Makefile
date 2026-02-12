.PHONY: build run clean

BINARY=graphscope
MAIN=./cmd/graphscope

build:
	CGO_ENABLED=1 go build -o $(BINARY) $(MAIN)

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)

tidy:
	go mod tidy
