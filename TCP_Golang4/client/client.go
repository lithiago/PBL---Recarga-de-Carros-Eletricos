package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
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
	Conn net.Conn
    Distancia float64 `json:"distancia"`
    Disponivel bool    `json:"disponivel"`
    Latitude float64    `json:"latitude"`
    Longitude float64    `json:"longitude"`

}

type Mensagem struct{
	Tipo string `json:"tipo"`
	Conteudo []byte `json:"conteudo"`
	OrigemMensagem string `json:"origemmensagem"`
}

// Construtor para criar um novo cliente
func NewClient(host string, port string) (*Client, error) {
	address := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao servidor: %v", err)
	}

	type MensagemInicializacao struct {
		Msg string `json:"msg"`
	}
	
	mensagemInicial := MensagemInicializacao{Msg: "Inicio de Conexão"}
	conteudoJSON, err := json.Marshal(mensagemInicial)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar mensagem inicial: %v", err)
	}

	req := Mensagem{
		Tipo:           "Conexao",
		Conteudo:       conteudoJSON,
		OrigemMensagem: "CARRO",
	}

	dados, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar mensagem: %v", err)
	}

	if _, err := conn.Write(append(dados, '\n')); err != nil {
		return nil, fmt.Errorf("erro ao enviar mensagem inicial: %v", err)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	minLat, maxLat := -23.6, -23.5
	minLong, maxLong := -46.7, -46.6

	return &Client{
		conn:      conn,
		reader:    bufio.NewReader(conn),
		writer:    bufio.NewWriter(conn),
		latitude:  randomInRange(r, minLat, maxLat),
		longitude: randomInRange(r, minLong, maxLong),
		bateria:   100, // Bateria começa cheia
		msgChan:   make(chan Mensagem, 10),
	}, nil
}


// Função para gerar um número aleatório dentro de um intervalo
func randomInRange(r *rand.Rand, min, max float64) float64 {
	return min + r.Float64()*(max-min)
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

// Função para solicitar pontos de recarga ao servidor
func (c *Client) solicitaPontos() error {
	// Definição da estrutura interna da requisição
	type ReqPontoDeRecarga struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	// Criar o objeto com os valores
	req := ReqPontoDeRecarga{
		Latitude:  c.latitude,
		Longitude: c.longitude,
	}

	// Serializar o JSON da requisição
	conteudoJSON, err := json.Marshal(req)
	if err != nil {
		log.Println("Erro ao serializar reserva:", err)
		return err
	}

	// Criar a mensagem principal
	mensagem := Mensagem{
		Tipo:           "Pontos",
		Conteudo:       conteudoJSON,
		OrigemMensagem: "CARRO",
	}

	// Serializar a mensagem para JSON
	dados, err := json.Marshal(mensagem)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %v", err)
	}

	// Garantir que há um delimitador no final para facilitar a leitura do servidor
	dados = append(dados, '\n')

	// Enviar a mensagem diretamente pelo socket
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, err = c.conn.Write(dados) // Escreve diretamente no socket
	if err != nil {
		return fmt.Errorf("erro ao enviar dados: %v", err)
	}

	return nil
}



func (c *Client) solicitarReserva(posicaoPonto int, pontos[]PontoRecarga){

	type Reserva struct{
		Ponto PontoRecarga `json:"ponto"`
	}
	reserva := Reserva{Ponto: pontos[posicaoPonto]}

	conteudoJSON, err := json.Marshal(reserva)
	if err != nil {
        log.Println("Erro ao serializar reserva:", err)
        
    }
	msg := Mensagem{
        Tipo:     "RESERVA",
        Conteudo: conteudoJSON,
    }
	dados, err := json.Marshal(msg)
    if err != nil {
        log.Println("Erro ao serializar mensagem:", err)
        return
    }

    c.Send(string(dados)) // Envia a string JSON pelo socket
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

// Função para movimentar o carro
// Quando finalizar a recarga volta a movimentar o carro
func (c *Client) movimentarCarro(ctx context.Context) {
	// Definindo a velocidade do carro (em km/h)
	const velocidade = 10.0 // Velocidade constante
	const intervalo = 1.0   // Intervalo de tempo em segundos para movimentação

	// Gerador de números aleatórios
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Movimentação aleatória até a bateria ficar crítica
	for {
		select {
		case <-ctx.Done():
			return // Sai da goroutine se o contexto for cancelado
		default:
			if c.bateria > 20 {
				// Gera uma direção aleatória
				direcao := r.Float64() * 360 // Direção em graus

				// Simula a movimentação
				c.latitude += velocidade * (math.Cos(direcao*math.Pi/180) * intervalo / 100)  // Atualiza latitude
				c.longitude += velocidade * (math.Sin(direcao*math.Pi/180) * intervalo / 100) // Atualiza longitude

				// Espera o intervalo
				time.Sleep(time.Duration(intervalo * float64(time.Second))) // Espera o intervalo
			} else {
				// Se a bateria estiver crítica, pode-se sair do loop ou parar a movimentação
				break
			}
		}
	}
}

func (c *Client) movimentarParaPonto(latitudePonto, longitudePonto float64) {
	// Simula a movimentação em direção ao ponto de recarga
	for c.latitude != latitudePonto || c.longitude != longitudePonto {
		// Calcula a direção para o ponto de recarga
		direcao := math.Atan2(longitudePonto-c.longitude, latitudePonto-c.latitude) * 180 / math.Pi

		// Atualiza a posição do carro
		c.latitude += 0.01 * (math.Cos(direcao * math.Pi / 180))  // Ajuste a taxa de movimento
		c.longitude += 0.01 * (math.Sin(direcao * math.Pi / 180)) // Ajuste a taxa de movimento

		// Espera um pouco antes de continuar a movimentação
		time.Sleep(100 * time.Millisecond)
	}

	// Ao chegar no ponto de recarga, iniciar o processo de recarga
	c.iniciarRecarga()
}

func (c *Client) iniciarRecarga(){
	//
}

func (c *Client) processarMensagens(msg Mensagem){
	for {
		
		switch msg.Tipo {
		case "PONTOS":
			var pontos []PontoRecarga
			var opcao int
			if err := json.Unmarshal(msg.Conteudo, &pontos); err != nil {
				log.Println("Erro ao decodificar pontos:", err)
				continue
			}
			c.mostrarPontos(pontos)
			fmt.Print(" 👉 Escolha um Ponto para reservar: ")
			fmt.Scanln(&opcao)
			c.solicitarReserva(opcao, pontos)

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
	var i int = 0
	for _, p := range pontos {
		fmt.Printf("[%d] -> (%.2f Metros) - Disponível: %v\n", i, p.Distancia, p.Disponivel)
		i +=1
	}
}

func (c *Client) mostrarStatusReserva(status bool) {
	if status {
		fmt.Println("\n✅ Reserva confirmada!")
	} else {
		fmt.Println("\n❌ Falha na reserva!")
	}
}

func (c *Client) receberMensagem() {
 //   fmt.Println("[DEBUG] Goroutine receberMensagem iniciada!")

    if c.conn == nil {
        log.Println("[ERRO] Conexão é nula! Encerrando goroutine.")
        return
    }

    reader := bufio.NewReader(c.conn)

    for {
   //     fmt.Println("[DEBUG] Esperando dados do servidor...")

        respostaBytes, err := reader.ReadBytes('\n')
        if err != nil {
            log.Println("[ERRO] Falha ao ler do servidor:", err)
            break // encerra o loop se a conexão for perdida
        }

     //   fmt.Println("[DEBUG] Dados recebidos:", string(respostaBytes))

        var resposta Mensagem
        err = json.Unmarshal(respostaBytes, &resposta)
        if err != nil {
            log.Println("[ERRO] Falha ao decodificar JSON:", err)
            continue
        }

       // fmt.Println("[DEBUG] Mensagem decodificada, Tipo:", resposta.Tipo)

        // Confirma se o canal ainda está aberto antes de enviar
        select {
        case c.msgChan <- resposta:
        //    fmt.Println("[DEBUG] Mensagem enviada para msgChan:", resposta)
        default:
        //    log.Println("[ERRO] Canal msgChan está bloqueado! Mensagem perdida:", resposta)
        }
    }

    log.Println("[DEBUG] Goroutine receberMensagem ENCERRADA")
}


func (c *Client) trocaDeMensagens() {
	contexto, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Inicia as goroutines para monitoramento e recebimento de mensagens
	go c.monitorarBateria(contexto)
	go c.receberMensagem()
	entradaChan := make(chan string) // Canal para capturar a entrada do usuário
	go func() {
		for {
			var opcao string
			fmt.Scanln(&opcao)
			entradaChan <- opcao
		}
	}()

	for {
		// Exibe o menu apenas quando o usuário pode interagir
		fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("          🚀 MENU PRINCIPAL 🚀        ")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("  1️⃣  | Solicitar Pontos de Recarga")
		fmt.Println("  2️⃣  | Encerrar Conexão")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Print(" 👉 Escolha uma opção: ")

		select {
		case <-contexto.Done():
			return

		case msg := <-c.msgChan:
			fmt.Println(msg.Tipo)
			c.processarMensagens(msg)

		case opcao := <-entradaChan:
			switch opcao {
			case "1":
				limparTela()
				if err := c.solicitaPontos(); err != nil {
					fmt.Println("Erro ao solicitar pontos:", err)
				}

			case "2":
				fmt.Println("🔌 Encerrando conexão...")
				c.Send("Sair")
				cancel() // Cancela o contexto para interromper as goroutines
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

