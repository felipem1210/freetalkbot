# freetalkbot
Your own IA to handle communications with your customers via phonecalls or whatsapp

## Configure asterisk

1. Run `docker-compose up -d` to bootstrap asterisk.
2. Copy local-config to container-config `cp -a asterisk/local-config/* asterisk/container-config/`
3. Restart asterisk container `docker-compose restart asterisk`

Asterisk is raised up in network_mode host.

## Register SIP endpoint

Checkout pjsip_endpoint.conf file.

## Run talkbot server

Go version recommended: 1.22

Raise up server

```sh
go mod tidy
go run main.go init -c audio
```

For better golang developer experience you can install [golang-air](https://github.com/cosmtrek/air) and initialize the talkbot server with this:

```sh
air -- init -c audio
```
