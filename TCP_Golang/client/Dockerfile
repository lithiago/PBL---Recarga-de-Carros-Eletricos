# Etapa de construção
FROM golang:1.19-alpine AS builder

# Definir o diretório de trabalho dentro do container
WORKDIR /app

# Copiar o código do cliente para o container
COPY client/client.go .

# Compilar o código Go
RUN go mod init client && go build -o client

# Etapa final
FROM alpine:latest

# Definir o diretório de trabalho
WORKDIR /app

# Copiar o binário compilado do cliente
COPY --from=builder /app/client .

# Comando para rodar o cliente
CMD ["./client"]
