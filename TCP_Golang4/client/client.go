	package main

	import (
		"bufio"
		"encoding/json"
		"fmt"
		"log"
		"net"
		"os"
		"os/exec"
		"runtime"
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
		_, err := c.conn.Write([]byte(message + "\n")) // Adiciona quebra de linha para delimitar
		return err
	}

	// Solicitar pontos de recarga ao servidor
	func (c *Client) solicitaPontos() error {
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


	func (c *Client) monitorarBateria() {
		for {
			if c.bateria <= 20 {
				fmt.Println("Bateria crítica! Enviando solicitação ao servidor...")
				if err := c.solicitaPontos(); err != nil {
					fmt.Println("Erro ao solicitar pontos:", err)
				}
				break // Para o monitoramento após a solicitação
			}
			time.Sleep(10 * time.Second) // Verifica a bateria a cada 10 segundos
		}
	}

	func (c *Client) iniciarRecarga(){
		//
	}

	// Loop de interação do cliente com o servidor
	func (c *Client) trocaDeMensagens() {
		for {
			fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("          🚀 MENU PRINCIPAL 🚀        ")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("  1️⃣  | Solicitar Pontos de Recarga")
			fmt.Println("  2️⃣  | Encerrar Conexão")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Print(" 👉 Escolha uma opção: ")

			var opcao string
			fmt.Scanln(&opcao)
			var opcaoDePonto string

			switch opcao {
			case "1":
				limparTela()
				if err := c.solicitaPontos(); err != nil {
					fmt.Println("Erro ao solicitar pontos:", err)
					continue
				}

				mapaDePontos, err := c.receberPontos()
				if err != nil {
					fmt.Println("Erro ao receber pontos:", err)
					continue
				}
				fmt.Printf("📌 Pontos disponíveis:\n")
				
				for k, v := range mapaDePontos{
					fmt.Printf("%s: %.2f\n", k, v)
				}
				fmt.Printf("Selecione um dos pontos: ")
				fmt.Scanln(&opcaoDePonto)
				c.solicitarReserva(opcaoDePonto)

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
