package main

import (
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




func main() {
	// Definindo o endereço do servidor
	client, err := NewClient("server", "3000")
	if err != nil {
		fmt.Println("Erro ao conectar ao servidor:", err)
		os.Exit(1)
	}

	// Mensagem a ser enviada
	
	err = client.Send("Olá, servidor!")


	if err != nil {
		fmt.Println("Erro ao enviar dados:", err)
		os.Exit(1)
	}


	fmt.Println("Mensagem enviada para o servidor!")
}
