package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	// Definindo o endereço do servidor
	serverAddress := "server:3000"
	
	// Estabelecendo a conexão TCP com o servidor
	conn, err := net.Dial("tcp", serverAddress)
	if err != nil {
		fmt.Println("Erro ao conectar ao servidor:", err)
		os.Exit(1)
	}

	// Mensagem a ser enviada
	message := "Olá, servidor!"

	// Enviando a mensagem para o servidor
	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Println("Erro ao enviar dados:", err)
		os.Exit(1)
	}

	// Espera para garantir que a mensagem seja recebida pelo servidor
	time.Sleep(1 * time.Second)

	fmt.Println("Mensagem enviada para o servidor:", message)
}
