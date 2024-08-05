SHELL := /bin/zsh

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