.PHONY:
build:
	docker-compose build

.PHONY:
run: build
	docker-compose up