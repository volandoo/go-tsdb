.PHONY: build run clean docker-build docker-run

build:
	cd src && go build -o main main.go

run:
	cd src && go run main.go

clean:
	cd src && rm -f main

docker-build:
	cd src && docker build -t tsdb .

docker-run: docker-build
	docker run --rm -p 1985:1985 -v $(shell pwd)/.data:/app/.data  tsdb

