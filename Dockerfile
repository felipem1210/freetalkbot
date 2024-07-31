# Usar una imagen base de Go para la etapa de construcción
FROM golang:1.22-bookworm AS builder

# Instalar gcc y otras herramientas necesarias para CGO
RUN apt-get update && apt-get install -y gcc

# Establecer el directorio de trabajo dentro del contenedor
WORKDIR /app

# Copiar los archivos go.mod y go.sum y descargar las dependencias
COPY go.mod go.sum ./
RUN go mod tidy

# Copiar el código fuente al directorio de trabajo
COPY . .

# Habilitar CGO y construir el binario para la aplicación principal
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
RUN go build -tags sqlite_omit_load_extension -o /freetalkbot main.go

# Crear una imagen mínima para ejecutar el binario
FROM debian:bookworm-slim

# Instalar las dependencias necesarias para la ejecución
RUN apt-get update && apt-get install -y ca-certificates tzdata sqlite3 gcc

# Establecer el directorio de trabajo dentro del contenedor
WORKDIR /root/

# Copiar el binario desde la etapa de construcción
COPY --from=builder /freetalkbot .

# Copiar archivos de configuración y scripts necesarios
COPY --from=builder /app/.env ./.env

# Exponer los puertos que utilizará la aplicación
EXPOSE 8080
EXPOSE 443

# Comando por defecto para ejecutar la aplicación
CMD ["./freetalkbot"]
