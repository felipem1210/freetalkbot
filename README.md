# freetalkbot

Implementation of VoIP/Whatsapp communication channels to interact with LLM/NLU bot assistants.

## VoIP channel

Audiosocket server receiving a request from Asterisk.

### Features:

* Simulates a real conversation, but instead of human you are talking with an assistant.
* If you don't want to hear more assistant answer you can talk back. The assistant voice will be cut and it will process what you talked.
* Supports multiple calls (in theory, I haven't had the chance to test this).
* Fast answer from assistant (Speed is limited by the STT tool transcription generation and assistant answer generation times).

### Architecture

Refer to [architecture-Voicebot.png](docs/architecture-Voicebot.png).

### Asterisk implementation

The request can be implemented in two ways:

1. Using [Audiosocket Dialplan application](https://docs.asterisk.org/Asterisk_20_Documentation/API_Documentation/Dialplan_Applications/AudioSocket/):

```sh
[dp_entry_call_inout]
exten = 101,1,Verbose("Call to AudioSocket via Channel interface")
same = n,Answer()
same = n,AudioSocket(40325ec2-5efd-4bd3-805f-53576e581d13,<audiosocketserver.address.com>:8080)
same = n,Hangup()
```

When using this way, the audio received from asterisk will be signed linear, 16-bit, 8kHz, mono PCM (little-endian). The envar `AUDIO_FORMAT` value must be `pcm16`.

2. Using [Audiosocket Channel driver](https://docs.asterisk.org/Configuration/Channel-Drivers/AudioSocket/)

```sh
[dp_entry_call_inout]
exten = 101,1,Verbose("Call to AudioSocket via Channel interface")
same = n,Answer()
same = n,Dial(AudioSocket/<audiosocketserver.address.com>:8080/40325ec2-5efd-4bd3-805f-53576e581d13)
same = n,Hangup()
```

When using this way, the audio received from asterisk will be use the codec negotiated between the phone and asterisk. By default it is g711, and the audiosocket server can process audio in this codec (both ulaw and alaw.). The envar `AUDIO_FORMAT` value must be `g711` and the envar `G711_AUDIO_CODEC` must be set between `ulaw` or `alaw`.
If you want to choose a different codec than `g711` you can, both you will have to implement the transformation of the audio data from that codec to `pcm16`. Please refer to [g711.go](packages/audiosocket/g711.go) file. 

### STT

There are two choices. 
* OpenAI Whisper or 
* Host [Faster Whisper Server](https://github.com/fedirz/faster-whisper-server). Second choice is recommended if you have GPU power. The advantage of using this server is that the audio is streamed via websocket protocol, which will guarantee more speed in transcription generation.

### TTS

It uses PicoTTS(https://github.com/ihuguet/picotts). The voices used are the ones that comes with pico.

### Languages supported 

They are limited by the languages that PicoTTS supports: en-EN, en-GB, es-ES, de-DE, fr-FR, it-IT

## WhatsApp channel

This implementation was done using [whatsmeow](https://pkg.go.dev/go.mau.fi/whatsmeow) library. **NO need of WhatsApp Business account, 100% free.**

### Features

* Free whatsapp server that acts like WhatsApp web.
* Conversations with the users via text or voice messages. For voice, the user sends it, and server returns text answer.
* It answers in the same language that the user. All languages supported!!.

### Architecture

Refer to [architecture-Whatsapp.png](docs/architecture-Whatsapp.png).

### Implementation

For this channel you will need a phone with WhatsApp installed and with a number. The server will act as a WhatsApp client that will pair with your WhatsApp account. 
After initialize the server you will see in the logs a QR code. Scan that QR code with the WhatsApp account that you will use.
If you can't scan the QR code you can also link the WhatsApp account using a pair code. For that you must set the envar `PAIR_PHONE_NUMBER` with your phone number using format show in the `.env.example`. If you don't need the pair code don't set this envar.

Once you pair your WhatsApp account the session will be stored in a sqlite file. This file is created inside the container but mapped through a docker volume, so you can use it when you want to develop locally. If you delete this file you will have to login again using a new QR code.

### STT Tool

When receiving an audio message it uses an STT tool to transcribe. It can be the same already mentioned in the VoIP channel.

### Languages supported

All languages that you want!!!

## Assistants Integration

Currently the channels are integrated with two LLM/NLU assistants.

* [RASA](./assistants/rasa/README.md)
* [Anthropic](./assistants/anthropic/README.md)

## Dependencies

* Golang. Version recommended: 1.22
* Golang packages. Check [go.mod](./go.mod) file
* [whatsapp-media-decrypt](https://github.com/ddz/whatsapp-media-decrypt/tree/master) tool
* [picotts](https://github.com/ihuguet/picotts)

Install go dependencies with `go mod tidy`. Run it as well if you add a new package

### Environment variables

Check the variables in `env.example` file. There you will have a detailed description of each variable to setup the communications channels with the STT tool and assistant of your choice. Create `.env` file with `cp -a .env.example .env` and modify it with your values. 
Read carefully the file to know which variables are relevant for each component

## Run

You can pull the docker image and run it with the environment variables set up. Choose your communication channel between whatsapp or audio

```sh
docker pull ghcr.io/felipem1210/freetalkbot/freetalkbot:latest
COM_CHANNEL=audio #or whatsapp
docker run -it --rm --env-file ./.env ghcr.io/felipem1210/freetalkbot/freetalkbot:latest freetalkbot init -c $COM_CHANNEL
```

## Development

For local development you can use docker or podman to raise up the components defined in the `docker-compose.yml` file. These components are:

* Asterisk
* Anthropic connector
* Rasa assistant
* Rasa Actions server
* Faster Whisper Server (optional)
* Audio bot server
* Whatsapp bot server

### Build docker images

Run `make build`. This will build locally all the images needed for components.

### Run the solution

After setting up properly the environment variables:

* Without faster-whisper-server: `make run`
* With faster-whisper-server using cpu: `make run-local-whisper-cpu`
* With faster-whisper-server using gpu: `make run-local-whisper-gpu`

### Configure asterisk

1. Once raised up, copy local-config to container-config `cp -a asterisk/local-config/* asterisk/container-config/`
2. Restart asterisk container `docker-compose restart asterisk`

Asterisk is raised up in network_mode brige. The asterisk configuration files are mapped in folder `asterisk/container-config`

### Register SIP/IAX endpoint in a softphone

* For SIP checkout `pjsip_endpoint.conf` file in `asterisk/container-config` folder.
* For IAX checkout iax.conf file in `asterisk/local-config` folder.

# Gratitude and Thanks

The following projects inspired to the construction of this one:

* [audiosocket](https://github.com/CyCoreSystems/audiosocket)
* [whatsmeow-quickstart](https://github.com/codespearhead/whatsmeow-quickstart)
* [faster-whisper-server](https://github.com/fedirz/faster-whisper-server)