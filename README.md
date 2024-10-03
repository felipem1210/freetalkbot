# freetalkbot

Implementation of communication channels to interact with LLM/NLU bot assistants.

* **Voice:** using [Audiosocket Asterisk](https://docs.asterisk.org/Configuration/Channel-Drivers/AudioSocket/) protocol
* **Whatsapp:** using [whatsmeow](https://github.com/tulir/whatsmeow) library. NO need of Whatsapp Business account, 100% free.

## Dependencies

* Golang. Version recommended: 1.22
* Golang packages. Check [go.mod](./go.mod) file
* [whatsapp-media-decrypt](https://github.com/ddz/whatsapp-media-decrypt/tree/master) tool
* [picotts](https://github.com/ihuguet/picotts)

Install go dependencies with `go mod tidy`. Run it as well if you add a new package

### Environment variables

Check the variables in `env.example` file. Create `.env` file with `cp -a .env.example .env` and modify it with your values. 
Read carefully the file to know which variables are relevant for each component

## Run

You can pull the docker image and run it with the environment variables set up. Choose your communication channel between whatsapp or audio

```sh
docker pull ghcr.io/felipem1210/freetalkbot/freetalkbot:latest
COM_CHANNEL=audio #or whatsapp
ocker run -it --rm --env-file ./.env ghcr.io/felipem1210/freetalkbot/freetalkbot:latest freetalkbot init -c $COM_CHANNEL
```

## Development

For local development you can use docker or podman to raise up the components defined in the `docker-compose.yml` file. These components are:

* Asterisk
* Anthropic
* Rasa assistant
* Rasa Actions server
* [Whisper ASR](https://ahmetoner.com/whisper-asr-webservice/) (optional)
* Audio bot server
* Whatsapp bot server

### Build docker images

Run `make build`. This will build locally all the images needed for components.

### Run the solution

After setting up properly the environment variables:

* Without whisper-asr: `make run`
* With whisper-local using cpu: `make run-local-whisper-cpu`
* With whisper-local using gpu: `make run-local-whisper-gpu`

### Configure asterisk

1. Once raised up, copy local-config to container-config `cp -a asterisk/local-config/* asterisk/container-config/`
2. Restart asterisk container `docker-compose restart asterisk`

Asterisk is raised up in network_mode brige. The asterisk configuration files are mapped in folder `asterisk/container-config`

### Register SIP/IAX endpoint in a softphone

* For SIP checkout `pjsip_endpoint.conf` file in `asterisk/container-config` folder.
* For IAX checkout iax.conf file in `asterisk/local-config` folder.

## Communication Channels

You can communicate with your chatbot assistant via two channels.

### Voice channel

Audiosocket server implementation, receives a request from Asterisk.

### Whatsapp channel

Same variables than audio bot are needed, just change the make command `make run-local-whatsapp`

After initialize you will see in the logs a QR code. Scan that QR code with the whatsapp account that you will use.
If you can't scan the QR code you can also link the whatsapp account using a pair code. For that you must set the envar `PAIR_PHONE_NUMBER` with your phone number using format show in the `.env.example`. If you don't need the pair code don't set this envar.

Once you pair your whatsapp account the session will be stored in a sqlite file. This file is created inside the container but mapped through a docker volume, so you can use it when you want to develop locally. If you delete this file you will have to login again using a new QR code.

The channel is prepared to receive text or voice messages.

## Assistants

Currently the channels are integrated with two LLM/NLU assistants.

* [RASA](./rasa/README.md)
* [Anthropic](./anthropic/README.md)

# Gratitude and Thanks

The following projects inspired to the construction of this one:

* [audiosocket](https://github.com/CyCoreSystems/audiosocket)
* [whatsmeow-quickstart](https://github.com/codespearhead/whatsmeow-quickstart)
* [faster-whisper-server](https://github.com/fedirz/faster-whisper-server)