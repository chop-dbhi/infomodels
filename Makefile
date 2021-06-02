BUILD_VERSION=$(shell git log -1 --pretty=format:"%h (%ci)")
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

build:
	mkdir -p dist

	go build \
		-o "dist/$(GOOS)-$(GOARCH)/infomodels" \
		-ldflags "-X 'main.progBuild=$(BUILD_VERSION)' -extldflags '-static'"

zip-build:
	cd dist && zip infomodels-$(GOOS)-$(GOARCH).zip $(GOOS)-$(GOARCH)/*

dist:
	GOOS=darwin GOARCH=amd64 make build zip-build
	GOOS=linux GOARCH=amd64 make build zip-build
	GOOS=windows GOARCH=amd64 make build zip-build

.PHONY: dist
