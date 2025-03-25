package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
)


type Client struct{
	conn net.Conn // Conexão TCP
	latitude float32
	longitude float32
	id string
	bateria int
}


func NewClient(host string, port string) (*Client, error){
	address := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", address)
	if err != nil{
		return nil, err
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Send(message string) error {
	_, err := c.conn.Write([]byte(message))
	return err
}


func (c *Client) solicitaPontos() (map[string]float64, error) {
	// Enviar a solicitação com os parâmetros
	mensagem := fmt.Sprintf("Pontos %.6f %.6f\n", c.latitude, c.longitude)
	_, err := fmt.Fprint(c.conn, mensagem)
	if err != nil {
		return nil, fmt.Errorf("erro ao enviar solicitação: %v", err)
	}

	// Ler a resposta do servidor
	resposta, err := bufio.NewReader(c.conn).ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("erro ao receber resposta: %v", err)
	}

	// Converter JSON para map[string]float64
	var pontos map[string]float64
	err = json.Unmarshal([]byte(resposta), &pontos)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON: %v", err)
	}

	return pontos, nil
}


func main() {
	// Definindo o endereço do servidor
	client, err := NewClient("server", "3000")
	if err != nil {
		fmt.Println("Erro ao conectar ao servidor:", err)
		os.Exit(1)
	}
	if err != nil {
		fmt.Println("Erro ao enviar dados:", err)
		os.Exit(1)
	}

	pontos, _ := client.solicitaPontos()

	if err != nil {
		fmt.Println("Erro:", err)
		return
	}

	// Exibir pontos recebidos
	fmt.Println("Pontos de Recarga:", pontos)
}
