.PHONY: build clean fmt vet test tidy

build:
	go build -o nomad-scheduler-analyzer ./cmd/main.go

clean:
	rm -f nomad-scheduler-analyzer main

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy
