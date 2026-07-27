# Makefile for the mahshad mini-container project.

BINARY = mahshad-container
NEWROOT = ./new-root

# Build the Go program into a single binary.
build:
	go build -o $(BINARY) .

# Build and run an interactive shell inside the container.
run: build
	sudo ./$(BINARY) --rootfs $(NEWROOT) --hostname mahshad /bin/sh

# Run with resource limits (100 MB memory, half a CPU core).
run-limits: build
	sudo ./$(BINARY) --rootfs $(NEWROOT) --hostname mahshad \
		--memory 100m --cpu 0.5 /bin/sh

# Run the test script.
test: build
	sudo ./test.sh

# Remove the built binary and any leftover cgroups.
clean:
	rm -f $(BINARY)
	-sudo rmdir /sys/fs/cgroup/mahshad-* 2>/dev/null || true

.PHONY: build run run-limits test clean
