run-audio:
	air -- init -c audio

run-whatsapp:
	air -- init -c whatsapp

rasa-train:
	docker-compose exec rasa rasa train
	docker-compose restart rasa