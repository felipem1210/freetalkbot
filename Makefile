SHELL := /bin/zsh

build:
	COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 docker-compose build

run:
	docker-compose up -d

run-whisper-asr:
	docker-compose --profile whisper-asr up -d

run-local-audio:
	docker-compose down
	docker-compose up -d
	docker-compose stop gobot_voip
	air -- init -c audio

run-local-whatsapp:
	docker-compose down
	docker-compose up -d
	docker-compose stop gobot_whatsapp
	air -- init -c whatsapp

rasa-train:
	docker-compose exec rasa rasa train
	docker-compose restart rasa