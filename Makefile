.PHONY: build run test clean generate program-build program-docker-build

build:
	cd app && go build -o ../solock ./cmd/solock/

run: build
	./solock

test:
	cd app && go test -count=1 ./internal/...

generate:
	cd app && go generate ./internal/mock/

program-docker-build:
	docker build --platform linux/amd64 -t solock-builder -f docker/Dockerfile .

program-build: program-docker-build
	mkdir -p app/internal/repository/adapter/program
	docker run --platform linux/amd64 --rm -v $(CURDIR)/app/internal/repository/adapter/program:/output solock-builder

clean:
	rm -f solock
