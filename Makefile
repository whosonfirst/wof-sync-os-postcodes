.PHONY: build
build:
	go build -mod vendor -o bin/wof-sync-os-postcodes cmd/wof-sync-os-postcodes/main.go