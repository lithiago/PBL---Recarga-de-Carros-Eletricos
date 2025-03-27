package main

import (
	"fmt"
	"net"
)

type Ponto struct {
	conn            net.Conn
	latitude        float64
	longitude       float64
	fila            []string
	disponibilidade bool
}

func NewPosto(host string, port string) (*Ponto, error) {
	address := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	return &Ponto{
		conn:            conn,
		latitude:        -23.5505, // Exemplo: São Paulo
		longitude:       -46.6333, // Exemplo: São Paulo
		disponibilidade: true,     // Inicializa como disponível
	}, nil
}

// Fechar conexão
func (c *Ponto) Close() {
	c.conn.Close()
}

// Enviar mensagem para o servidor
func (c *Ponto) Send(message string) error {
	_, err := c.conn.Write([]byte(message + "\n")) // Adiciona quebra de linha para delimitar
	return err
}

// Receber solicitação do servidor
func (c *Ponto) receberSolicitacao() (string, error) {
	buffer := make([]byte, 4096)

	// Lê os dados do socket
	dados, err := c.conn.Read(buffer)
	if err != nil {
		return "error", fmt.Errorf("erro ao ler resposta do servidor: %v", err)
	}

	resposta := string(buffer[:dados])

	return resposta, nil
}

// Indicar Disponibilidade e fila
func (c *Ponto) solicitaPontos() error {
	// Formata os parâmetros para string
	var disp string
	if c.disponibilidade {
		disp = "disponível"
	} else {
		disp = "indisponível"
	}

	// Formatar a mensagem
	mensagem := fmt.Sprintf("Disponibilidade: %s, Latitude: %.6f, Longitude: %.6f, Fila: %v, Posto\n", disp, c.latitude, c.longitude, c.fila)

	// Enviar dados
	err := c.Send(mensagem)
	if err != nil {
		return fmt.Errorf("erro ao enviar solicitação: %v", err)
	}

	return nil
}

func main() {
	// Criar um novo ponto
	ponto, err := NewPosto("localhost", "8080")
	if err != nil {
		fmt.Println("Erro ao criar ponto:", err)
		return
	}
	defer ponto.Close()

}
