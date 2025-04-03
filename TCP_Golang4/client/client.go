package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// Estrutura do Cliente
type Client struct {
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	latitude  float64
	longitude float64
	bateria   int
	mutex sync.Mutex
	msgChan chan Mensagem
}

type PontosDeRecarga struct{
	Localizacao []PontoRecarga
}



type PontoRecarga struct {
    ID        string  `json:"id"`
    Nome      string  `json:"nome"`
    Distancia float64 `json:"distancia"`
    Disponivel bool    `json:"disponivel"`
}

type Mensagem struct{
	Tipo string `json:"tipo"`
	Conteudo json.RawMessage `json:"conteudo"`
}

// Construtor para criar um novo cliente
func NewClient(host string, port string) (*Client, error) {
	address := net.JoinHostPort(host, port)
	// Tag de Cliente no Address?
	conn, err := net.Dial("tcp", address)
	fmt.Fprintln(conn, "client")
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:      conn,
		reader:    bufio.NewReader(conn),
		writer:    bufio.NewWriter(conn),
		latitude:  -23.5505, // Exemplo: São Paulo
		longitude: -46.6333, // Exemplo: São Paulo
	}, nil
}

// Fechar conexão
func (c *Client) Close() {
	c.conn.Close()
}

// Enviar mensagem para o servidor
func (c *Client) Send(message string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, err := c.conn.Write([]byte(message + "\n")) // Adiciona quebra de linha para delimitar
	return err
}

// Solicitar pontos de recarga ao servidor
func (c *Client) solicitaPontos() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	// Formata os parâmetros para string
	mensagem := fmt.Sprintf("Pontos %.6f %.6f Carro %d\n", c.latitude, c.longitude, c.bateria)
	// Enviar dados
	err := c.Send(mensagem)
	if err != nil {
		return fmt.Errorf("erro ao enviar solicitação: %v", err)
	}

	return nil
}

// Receber pontos de recarga do servidor
func (c *Client) receberPontos() (map[string]float64, error) {
	buffer := make([]byte, 4096)

	// Lê os dados do socket
	dados, err := c.conn.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta do servidor: %v", err)
	}

	resposta := string(buffer[:dados])
	fmt.Println(resposta)
	var mapaPontos map[string]float64

	// Desserializa os dados corretamente
	err = json.Unmarshal([]byte(resposta), &mapaPontos)
	if err != nil {
		return nil, fmt.Errorf("erro ao desserializar JSON: %v", err)
	}

	return mapaPontos, nil
}

func (c *Client) solicitarReserva(ponto string){
	type Mensagem struct{
		PontoEscolhido string
		Cliente Client
	}
	msg := Mensagem{PontoEscolhido: ponto, Cliente: *c}
	dados, err := json.Marshal(msg)
	if err != nil {
		fmt.Println("Erro ao serializar mensagem:", err)
	}
	c.Send(string(dados))
}



// Como essa função vai se tratar de uma Goroutine é preciso que um contexto seja passado. E em go 
func (c *Client) monitorarBateria(contexto context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for{
		select {
		case <- contexto.Done():
			return
		case <- ticker.C:
			// Como outras rotinas compartilham o atributo Bateria, o mutex se torna necessário para evitar condições de corrida. Garantindo então que a Bateria seja acessado somente por uma rotina por vez
			c.mutex.Lock()
			bateriaAtual := c.bateria
			c.mutex.Unlock()
			if bateriaAtual <= 20 {
				fmt.Println("\n⚠️ Bateria crítica! Enviando solicitação ao servidor...")
				if err := c.solicitaPontos(); err != nil {
					fmt.Println("Erro ao solicitar pontos:", err)
				}
				
				// Diminui a bateria mais lentamente após o alerta
				c.mutex.Lock()
				if c.bateria > 5 { // Não deixa a bateria zerar
					c.bateria -= 2
				}
				c.mutex.Unlock()
			
		} else {
			c.mutex.Lock()
			c.bateria -= 5
			c.mutex.Unlock()
			}
		}
	}
}

func (c *Client) iniciarRecarga(){
	//
}

func (c *Client) processarMensagens(msg Mensagem){
	for {
		
		switch msg.Tipo {
		case "PONTOS":
			var pontos []PontoRecarga
			if err := json.Unmarshal(msg.Conteudo, &pontos); err != nil {
				log.Println("Erro ao decodificar pontos:", err)
				continue
			}
			c.mostrarPontos(pontos)

		case "RESERVA":
			var reserva struct {
				Status bool `json:"status"`
			}
			if err := json.Unmarshal(msg.Conteudo, &reserva); err != nil {
				log.Println("Erro ao decodificar reserva:", err)
				continue
			}
			c.mostrarStatusReserva(reserva.Status)
		}
	}
}

func (c *Client) mostrarPontos(pontos []PontoRecarga) {
	fmt.Println("\nPontos de recarga disponíveis:")
	for _, p := range pontos {
		fmt.Printf("- %s (%.2f km) - Disponível: %v\n", p.Nome, p.Distancia, p.Disponivel)
	}
}

func (c *Client) mostrarStatusReserva(status bool) {
	if status {
		fmt.Println("\n✅ Reserva confirmada!")
	} else {
		fmt.Println("\n❌ Falha na reserva!")
	}
}


func (c *Client) receberMensagem(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, err := c.reader.ReadBytes('\n')
				if err != nil {
					log.Println("Erro ao ler:", err)
					return
				}

				var msg Mensagem
				if err := json.Unmarshal(line, &msg); err != nil {
					log.Println("Erro ao decodificar:", err)
					continue
				}

				c.msgChan <- msg
			}
		}
	}()
}
// Loop de interação do cliente com o servidor
func (c *Client) trocaDeMensagens() {
	contexto, cancel := context.WithCancel(context.Background())
	defer cancel()

	c.receberMensagem(contexto)

	for {

		select{
		case <- contexto.Done():
			return
		case msg := <-c.msgChan:
			c.processarMensagens(msg)
		default:
		
			fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("          🚀 MENU PRINCIPAL 🚀        ")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("  1️⃣  | Solicitar Pontos de Recarga")
			fmt.Println("  2️⃣  | Encerrar Conexão")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Print(" 👉 Escolha uma opção: ")

			var opcao string
			fmt.Scanln(&opcao)
			switch opcao {
			case "1":
				limparTela()
				if err := c.solicitaPontos(); err != nil {
					fmt.Println("Erro ao solicitar pontos:", err)
					continue
				}

			case "2":
				fmt.Println("🔌 Encerrando conexão...")
				c.Send("Sair")
				c.Close()
				return

			default:
				fmt.Println("⚠️  Opção inválida. Tente novamente.")
			}
		}
	}
}

// Função para limpar o terminal
func limparTela() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}

	cmd.Stdout = os.Stdout
	cmd.Run()
}

func main() {
	client, err := NewClient("server", "3000")
	if err != nil {
		log.Fatal(err)
	}

	client.trocaDeMensagens()
}


/* A ideia central é usar goroutines para deixar seu programa fazendo várias coisas ao mesmo tempo de forma eficiente, sem travar. Imagine seu sistema de recarga de carros elétricos: enquanto o usuário está vendo o menu, o programa pode estar checando o nível da bateria em segundo plano e também ouvindo mensagens do servidor. 

Quando a bateria ficar baixa, o sistema automaticamente avisa o servidor sem precisar que o usuário faça nada. Se o servidor mandar uma lista de postos de recarga, isso aparece na tela sem congelar a interface. Tudo acontece de forma fluida, como um bom aplicativo de celular que continua respondendo mesmo quando está carregando dados.

A mágica está em dividir o trabalho em tarefas menores que rodam paralelamente: uma cuida da bateria, outra fica ouvindo o servidor, outra processa os dados recebidos. O contexto serve como um interruptor geral - se precisar fechar o programa, todas essas tarefas são avisadas para encerrar limpasmente, sem deixar nada pendurado. 

É como ter vários assistentes trabalhando juntos, cada um com sua função, mas coordenados pelo mesmo chefe (o contexto). Isso torna seu sistema mais rápido, responsivo e profissional, especialmente importante quando lida com operações de rede que podem demorar ou falhar. */