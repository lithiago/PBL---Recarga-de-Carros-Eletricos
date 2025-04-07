# ⚡🚗 Sistema Inteligente de Recarga de Carros Elétricos

Este repositório contém a implementação de um sistema cliente-servidor para comunicação inteligente entre veículos elétricos e pontos de recarga, desenvolvido em Go com sockets TCP nativos.

## 🎯 Objetivo

Facilitar o uso de carros elétricos oferecendo:

- Localização de pontos de recarga disponíveis;
- Reserva e liberação automática de carregadores;
- Distribuição inteligente da demanda para evitar filas;
- Registro de recargas com posterior pagamento eletrônico.

## 💻 Tecnologias e Requisitos

- Linguagem: Go (Golang)
- Comunicação: TCP/IP via sockets nativos (sem frameworks)
- Contêineres: Docker
- Execução local ou em nuvem (simulado)

## 📁 Estrutura

O sistema é dividido em três componentes principais:

- **Cliente (Veículo)**: solicita recarga e reserva pontos.
- **Servidor em Nuvem**: gerencia pontos e otimiza distribuição.
- **Ponto de Recarga**: controla disponibilidade e liberação.

## 🚀 Como Executar

1. Instale o [Docker](https://www.docker.com/get-started/) e o [Go](https://go.dev/doc/install).
2. Compile os arquivos `.go` de cada componente.
3. Utilize os arquivos Docker incluídos para subir múltiplas instâncias:
   ```bash
   docker-compose up --build
4. Digite para rodar o ponto:
  ```bash
   docker-compose run --rm ponto
   ```
5. Digite para rodar o cliente:
    ```bash
   docker-compose run --rm client
    ```

## 💸 Pagamentos

Todos os registros de recarga são associados a uma conta de usuário com suporte a pagamento via PIX.

## 📄 Requisitos e Restrições

- Comunicação apenas via sockets TCP;
- Sistema testado em contêineres Docker;
- Cada componente pode ser executado de forma independente.

## 📚 Relatório

O relatório completo em PDF está disponível no repositório com a justificativa da arquitetura e decisões técnicas adotadas.
