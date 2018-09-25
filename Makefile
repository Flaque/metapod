build:
	docker-compose build

.PHONY: run
run: build
	docker-compose up

prod:
	docker build ./db
	docker build ./api --build-arg app_env=production

test:
	docker build ./api --build-arg app_env=test