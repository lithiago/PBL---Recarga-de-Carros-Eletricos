# Etapa de construção
FROM golang:1.19-alpine AS builder

# Definir o diretório de trabalho dentro do container
WORKDIR /app

# Copiar o código do servidor para o container
COPY server/server.go .

# Compilar o código Go
RUN go mod init server && go build -o server

# Etapa final
FROM alpine:latest

# Definir o diretório de trabalho
WORKDIR /app

# Copiar o binário compilado do servidor
COPY --from=builder /app/server .

# Expôr a porta em que o servidor irá escutar
EXPOSE 3000

# Comando para rodar o servidor
CMD ["./server"]
