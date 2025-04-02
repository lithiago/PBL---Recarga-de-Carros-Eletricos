# Etapa de construção
FROM golang:1.19-alpine AS builder

# Definir o diretório de trabalho dentro do container
WORKDIR /app

# Copiar o código do servidor para o container
COPY Pontos/ponto.go .

# Compilar o código Go
RUN go mod init ponto && go build -o ponto

# Etapa final
FROM alpine:latest

# Definir o diretório de trabalho
WORKDIR /app

# Copiar o binário compilado do servidor
COPY --from=builder /app/ponto .


# Comando para rodar o servidor
CMD ["./ponto"]
