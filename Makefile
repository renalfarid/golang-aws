build:
	mkdir -p dist
	cp .env dist/
	go build -o dist/servermon