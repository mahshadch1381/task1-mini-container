# Makefile for the mahshad mini-container project.

BINARY = mahshad-container
NEWROOT = ./new-root

# Build the Go program into a single binary.
build:
	go build -o $(BINARY) .

# Build and run an interactive shell inside the container.
run: build
	sudo ./$(BINARY) --rootfs $(NEWROOT) --hostname mahshad /bin/sh

# Run the test script.
test: build
	sudo ./test.sh

# Remove the built binary.
clean:
	rm -f $(BINARY)

.PHONY: build run test clean
