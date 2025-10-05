
serve: 
	DOCKER_BUILDKIT=1 docker-compose up --build

build:
	@mkdir -p bin
	go build -o bin/app

ci: build
