SHELL := /bin/zsh


# Minimum versions
MINIMUM_PODMAN_VERSION = 4.0.0
MINIMUM_DOCKER_VERSION = 20.10.0

# Version comparsion func
version_ge = $(shell [ "$(printf '%s\n' $(1) $(2) | sort -V | head -n1)" != "$(1)" ] && echo true)

# Check Podman installed
PODMAN_PATH := $(shell command -v podman 2>/dev/null)
PODMAN_VERSION := $(shell podman --version 2>/dev/null | awk '{print $$3}')

# Check Docker installed
DOCKER_PATH := $(shell command -v docker 2>/dev/null)
DOCKER_VERSION := $(shell docker --version 2>/dev/null | awk '{print $$3}' | tr -d ,)

# Default CLI
CONTAINER_CLI =

ifeq ($(PODMAN_PATH),)
  ifeq ($(DOCKER_PATH),)
    $(error "Neither Podman nor Docker found!")
  else ifeq ($(call version_ge,$(MINIMUM_DOCKER_VERSION),$(DOCKER_VERSION)),true)
    CONTAINER_CLI = docker
  else
    $(error "Docker version $(DOCKER_VERSION) is too old! Minimum required: $(MINIMUM_DOCKER_VERSION)")
  endif
else ifeq ($(call version_ge,$(MINIMUM_PODMAN_VERSION),$(PODMAN_VERSION)),true)
  CONTAINER_CLI = podman
else
  $(error "Podman version $(PODMAN_VERSION) is too old! Minimum required: $(MINIMUM_PODMAN_VERSION)")
endif



build:
	COMPOSE_DOCKER_CLI_BUILD=1 DOCKER_BUILDKIT=1 $(CONTAINER_CLI) compose build

run:
	$(CONTAINER_CLI) compose up -d

run-whisper-asr:
	$(CONTAINER_CLI) compose --profile whisper-asr up -d

run-local-audio:
	$(CONTAINER_CLI) compose down
	$(CONTAINER_CLI) compose up -d
	$(CONTAINER_CLI) compose stop gobot_voip
	air -- init -c audio

run-local-whatsapp:
	$(CONTAINER_CLI) compose down
	$(CONTAINER_CLI) compose up -d
	$(CONTAINER_CLI) compose stop gobot_whatsapp
	air -- init -c whatsapp

rasa-train:
	$(CONTAINER_CLI) compose exec rasa rasa train
	$(CONTAINER_CLI) compose restart rasa
