# Etapa de construção
FROM golang:1.19-alpine AS builder

# Definir o diretório de trabalho dentro do container
WORKDIR /app

# Copiar o código do servidor para o container
COPY client/client.go .

# Compilar o código Go
RUN go mod init client && go build -o client

# Etapa final
FROM alpine:latest

# Definir o diretório de trabalho
WORKDIR /app

# Copiar o binário compilado do servidor
COPY --from=builder /app/client .


# Comando para rodar o servidor
CMD ["./client"]
