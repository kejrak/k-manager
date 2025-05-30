.PHONY: build run clean

build:
	go build -o bin/pod-error-monitor main.go

run: build
	./bin/pod-error-monitor

clean:
	rm -rf bin/

install-deps:
	go mod tidy 