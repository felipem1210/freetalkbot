# freetalkbot
Your own IA to handle communications with your customers via phonecalls or whatsapp

## Configure asterisk

1. Run `docker-compose up -d` to bootstrap asterisk.
2. Copy local-config to container-config `cp -a asterisk/local-config/* asterisk/container-config/`
3. Restart asterisk container `docker-compose restart asterisk`

Asterisk is raised up in network_mode host.

## Register SIP endpoint

Checkout pjsip_endpoint.conf file.

## Configure audiosocket-server

Go version recommended: 1.22

Raise up server

```sh
cd audiosocket-server
go mod tidy
go run main.go
```

For better golang developer experience you can install [golang-air](https://github.com/cosmtrek/air)
