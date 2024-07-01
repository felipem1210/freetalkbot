# freetalkbot
Your own IA to handle communications with your customers via phonecalls or whatsapp

## Configure asterisk

1. Run `docker-compose up -d` to bootstrap asterisk.
2. Copy local-config to container-config `cp -a asterisk/local-config/* asterisk/container-config/`
3. Restart asterisk container `docker-compose restart asterisk`

Asterisk is raised up in network_mode host.

## Register SIP endpoint

Checkout pjsip_endpoint.conf file.

## Dependencies

Go version recommended: 1.22
Install dependencies with `go mod tidy`

For better golang developer experience you can install [golang-air](https://github.com/cosmtrek/air)

## Run talkbot server

### Run audiosocket-server

```sh
go run main.go init -c audio
```

with air

```sh
air -- init -c audio
```

### Run whatsapp server

```sh
go run main.go init -c whatsapp
```

with air

```sh
air -- init -c whatsapp
```

After initialize you will see in the logs a QR code. Scan that QR code with the whatsapp account that you will use.
The whatsapp server store session in sqlite, so you will see a `examplestore.db` file. If you delete this file you will have to login using a new QR code.