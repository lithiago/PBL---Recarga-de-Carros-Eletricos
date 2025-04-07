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
	"strconv"
	"sync"
	"time"
)

// Estrutura do Cliente
type Client struct {
	conn      net.Conn 
	reader    *bufio.Reader
	writer    *bufio.Writer
	CoordenadaX  float64 `json:"coordenadaX"`
	CoordenadaY float64 `json:"coordenadaY"`
	Bateria   int	`json:"bateria"`
	Id int `json:"id"`
	mutex sync.Mutex
	msgChan chan Mensagem
	statusCarro string
	// Atributos para ficar a par do contexto das rotinas que montioram bateria e movimentação do carro
	movimentarCtx    context.Context 
	movimentarCancel context.CancelFunc
	// Atributos para manter o controle sobre a rotina que monitora a entrada do usuário
	entradaCtx    context.Context
	cancelarEntrada context.CancelFunc
	mensagemNoCanal bool
	RegistroDeCusto map[int]HistoricoDePagamento
}

type PontosDeRecarga struct{
	Localizacao []PontoRecarga
}


type HistoricoDePagamento struct{
	valor float64
	CoordenadaX float64
	CoordenadaY float64
}
type PontoRecarga struct {
    Distancia float64 `json:"distancia"`
    Disponivel bool    `json:"disponivel"`
    Latitude float64    `json:"latitude"`
    Longitude float64    `json:"longitude"`
	Id int `json:"id"`
	TempoEspera float64 `json:"tempo_espera"`

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
	minX, maxX := 0.0, 5000.0 // de 0 a 10 km no eixo X
	minY, maxY := 0.0, 5000.0 // de 0 a 10 km no eixo Y

	return &Client{
		conn:      conn,
		reader:    bufio.NewReader(conn),
		writer:    bufio.NewWriter(conn),
		CoordenadaX:  randomInRange(r, minX, maxX),
		CoordenadaY: randomInRange(r, minY, maxY),
		Bateria:   100, // Bateria começa cheia
		msgChan:   make(chan Mensagem, 10),
	}, nil
}

func (c *Client) setProcessando(valor bool) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    c.mensagemNoCanal = valor
}

func (c *Client) estaProcessando() bool {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    return c.mensagemNoCanal
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
		CoordenadaX  float64 `json:"coordenadaX"`
		CoordenadaY float64 `json:"coordenadaY"`
	}

	// Criar o objeto com os valores
	req := ReqPontoDeRecarga{
		CoordenadaX:  c.CoordenadaX,
		CoordenadaY: c.CoordenadaY,
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

func (c *Client) solicitarReserva(posicaoPonto int) {

	log.Println("Solicitando RESERVA")
	type Reserva struct {
		IdPonto int `json:"posicao"`
		Latitude float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Bateria   int	`json:"bateria"`
		Id int `json:"id"`
		
	}
	reserva := Reserva{IdPonto: posicaoPonto, Latitude: c.CoordenadaX, Longitude: c.CoordenadaY, Bateria: c.Bateria, Id: c.Id}

	conteudoJSON, err := json.Marshal(reserva)
	if err != nil {
		log.Println("Erro ao serializar reserva:", err)
		return
	}

	msg := Mensagem{
		Tipo:     "RESERVA",
		Conteudo: conteudoJSON,
		OrigemMensagem: "CARRO",
	}

	// Serializar a mensagem para JSON
	dados, err := json.Marshal(msg)
	if err != nil {
		log.Println("erro ao serializar mensagem: ", err)
	}

	// Garantir que há um delimitador no final para facilitar a leitura do servidor
	dados = append(dados, '\n')

	// Enviar a mensagem diretamente pelo socket
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, err = c.conn.Write(dados) // Escreve diretamente no socket
	if err != nil {
		log.Println("erro ao enviar dados: ", err)
	}

}



// Como essa função vai se tratar de uma Goroutine é preciso que um contexto seja passado. E em go 
func (c *Client) monitorarBateria(contexto context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-contexto.Done():
			return
		case <-ticker.C:
			c.mutex.Lock()
			bateriaAtual := c.Bateria
			c.mutex.Unlock()

			if bateriaAtual <= 20 {
				fmt.Println("\n⚠️ Bateria crítica! Enviando solicitação ao servidor...")
				if err := c.solicitaPontos(); err != nil {
					fmt.Println("Erro ao solicitar pontos:", err)
				}
			}
		}
	}
}


func (c *Client) iniciarMovimentacao() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.movimentarCancel != nil {
		c.movimentarCancel() // Cancela qualquer rotina anterior
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.movimentarCtx = ctx
	c.movimentarCancel = cancel

	go c.movimentarCarro(ctx)
	go c.monitorarBateria(ctx)
}

func (c *Client) pararMovimentacao() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.movimentarCancel != nil {
		c.movimentarCancel()
		c.movimentarCancel = nil
	}
}


// Modifique o movimentarCarro para verificar o contexto corretamente
func (c *Client) movimentarCarro(ctx context.Context) {
    const velocidade = 10.0
    const intervalo = 10.0
    r := rand.New(rand.NewSource(time.Now().UnixNano()))
    for {
        select {
        case <-ctx.Done():
            log.Println("Movimento aleatório cancelado")
            return
        default:
            c.mutex.Lock()
            if c.Bateria > 0 && c.statusCarro != "EM MOVIMENTO" {
                angulo := r.Float64() * 2 * math.Pi
                deltaX := velocidade * math.Cos(angulo) * intervalo
                deltaY := velocidade * math.Sin(angulo) * intervalo

                c.CoordenadaX += deltaX
                c.CoordenadaY += deltaY
                c.Bateria -= 1
                c.statusCarro = "MOVIMENTANDO"
				fmt.Printf("X: (%.2f, Y: %.2f) |\n", c.CoordenadaX, c.CoordenadaY)
            }
            c.mutex.Unlock()
            time.Sleep(time.Duration(intervalo * float64(time.Second)))
        }
    }
}


func (c *Client) movimentarParaPonto(destX, destY float64) {
	const (
		passoMetros     = 100.0              // Distância por passo
		distanciaMinima = 1.0              // Distância mínima para considerar "chegou"
		delay           = 1 * time.Second  // Intervalo entre passos
		tempoBateria    = 10 * time.Second // Tempo para consumir 1% de bateria
	)

	tempoAcumulado := time.Duration(0)

	for {
		deltaX := destX - c.CoordenadaX
		deltaY := destY - c.CoordenadaY
		distancia := math.Hypot(deltaX, deltaY)
	
		if distancia <= distanciaMinima {
			fmt.Printf("Destino alcançado: (%.2f, %.2f)\n", c.CoordenadaX, c.CoordenadaY)
			break
		}
	
		direcaoX := deltaX / distancia
		direcaoY := deltaY / distancia
	
		// Corrige para não ultrapassar o destino
		passo := math.Min(passoMetros, distancia)
	
		c.CoordenadaX += direcaoX * passo
		c.CoordenadaY += direcaoY * passo
	
		fmt.Printf("Movendo para (%.2f, %.2f) | Distância restante: %.2f | Bateria: %d%%\n",
			c.CoordenadaX, c.CoordenadaY, distancia, c.Bateria)
	
		time.Sleep(delay)
		tempoAcumulado += delay
	
		if tempoAcumulado >= tempoBateria {
			if c.Bateria > 0 {
				c.Bateria--
			}
			tempoAcumulado -= tempoBateria
		}
	}
	
}


func (c *Client) processarMensagens(msg Mensagem, entradaChan <-chan string) {

	c.setProcessando(true)  // Início do processamento
    defer c.setProcessando(false) // Garante que o flag volta para falso
	switch msg.Tipo {
	case "PONTOS":
		log.Println("eNTROU EM PROCESSAR MENSAGENS")
		var resposta struct {
			Listapontos []*PontoRecarga `json:"listapontos"`
		}
		var opcao int

		// Corrigido: decodifica a resposta com o campo "listapontos"
		if err := json.Unmarshal(msg.Conteudo, &resposta); err != nil {
			log.Println("Erro ao decodificar pontos:", err)
			
		}

		pontos := resposta.Listapontos
		c.mostrarPontos(pontos)

		fmt.Print(" 👉 Escolha um Ponto para reservar: ")
		opcaoStr := <-entradaChan
		opcao, err := strconv.Atoi(opcaoStr)
		if err != nil {
			log.Println("Entrada inválida:", err)
			return
		}
		c.solicitarReserva(opcao)

	case "RESERVA":
		log.Println("Estou Reservado")
		var reserva struct {
			Latitude float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			PontoId int `json:"pontoId"`

		}
		if err := json.Unmarshal(msg.Conteudo, &reserva); err != nil {
			log.Println("Erro ao decodificar reserva:", err)
			
		}
		fmt.Println("\n✅ Reserva confirmada!")
		c.pararMovimentacao()
		c.movimentarParaPonto(reserva.Latitude, reserva.Longitude)
	case "ID":
		log.Println("Entrou no ID")
		type dadosID struct {
			IdCliente int `json:"idCliente"`
		}

		var idRecebido dadosID
		if err := json.Unmarshal(msg.Conteudo, &idRecebido); err != nil {
			log.Println("Erro ao decodificar ID recebido:", err)
			return
		}
		c.Id = idRecebido.IdCliente
		log.Printf("✅ ID recebido e atribuído: %d\n", c.Id)
	// Essa case existe para que o cliente possa iniciar a recarga assim que chegar a vez
	case "AtualizacaoPosicaoFila":
		limparTela()
		var info struct {
			CarroId     int `json:"carroId"`
			PosicaoFila int `json:"posicaoFila"`
			PontoId     int `json:"pontoId"`
		}
		if err := json.Unmarshal(msg.Conteudo, &info); err != nil {
			log.Println("Erro ao decodificar posição na fila:", err)
			return
		}
		log.Println("Posição na Fila: ", info.PosicaoFila)
		if info.PosicaoFila == 0 {
			log.Printf("📍 Você está na posição %d da fila do ponto %d\n", info.PosicaoFila, info.PontoId)
	
			// Iniciar recarga se estiver em primeiro e ainda não estiver carregando
			if info.PosicaoFila == 0 {
				log.Println("⚡ Você está na posição 1. Iniciando recarga...")
				c.iniciarRecarga(info.PontoId)
			}
		}
	case "Pagamento":
		var Pagamento HistoricoDePagamento
		if err := json.Unmarshal(msg.Conteudo, &Pagamento); err != nil {
			log.Println("Erro ao decodificar posição na fila:", err)
			return
		}
		

	}
}

func (c *Client) mostrarPontos(pontos []*PontoRecarga) {
	fmt.Println("\nPontos de recarga disponíveis:")
	for _, p := range pontos {
		fmt.Printf("[%d] -> (%.2f Metros) - Tempo Até Carregamento: %.2f\n", p.Id, p.Distancia, p.TempoEspera)
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

func (c *Client)iniciarEntradaUsuario(entradaChan chan<- string) {
	c.entradaCtx, c.cancelarEntrada = context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-c.entradaCtx.Done():
				log.Println("[Entrada] Rotina de entrada finalizada.")
				return
			default:
				var opcao string
				_, err := fmt.Scanln(&opcao)
				if err != nil {
					log.Println("[Entrada] Erro ao ler:", err)
					continue
				}
				log.Println("[Entrada] Capturada:", opcao)
				entradaChan <- opcao
			}
		}
	}()
}
func (c *Client) iniciarRecarga(idPonto int) {
	log.Println("🔌 Iniciando processo de recarga...")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	totalCarregado := 0
	for range ticker.C {
		c.mutex.Lock()
		if c.Bateria >= 99 {
			c.Bateria = 100
			// Envia mensagem final de recarga concluída
			log.Println("✅ Bateria totalmente carregada.")
			c.enviarBateria(idPonto, totalCarregado, "RecargaConcluida")
			
			/* // Retorna à movimentação normal
			go c.iniciarMovimentacao()
			return */
		}

		// Simula incremento de 1% por segundo
		c.Bateria += 1
		totalCarregado++
		log.Printf("🔋 Bateria: %d%%\n", c.Bateria)
		c.mutex.Unlock()

		// Envia atualização da bateria
		c.enviarBateria(idPonto, totalCarregado, "Recarga")
	}
}


func (c *Client)enviarMensagem(conn net.Conn, msg Mensagem) {
	dados, err := json.Marshal(msg)
	if err != nil {
		log.Println("Erro ao serializar mensagem:", err)
		return
	}
	dados = append(dados, '\n')
	conn.Write(dados)
	log.Println("Servidor enviou os dados para o carro")
}

func (c *Client) enviarBateria(idPonto int, totalCarregado int, tipo string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	type bateria struct{
		Bateria int `json:"bateria"`
		CarroId      int `json:"carroId"`
		PontoId int `json:"pontoId"`		
	}

	conteudoJSON, err := json.Marshal(bateria{Bateria: c.Bateria, CarroId: c.Id, PontoId: idPonto})
	if err != nil {
		log.Println("Erro ao serializar reserva:", err)
		return
	}
	msg := Mensagem{
		Tipo:     tipo,
		Conteudo: conteudoJSON,
		OrigemMensagem: "CARRO",
	}
	c.enviarMensagem(c.conn, msg)
}


func (c *Client) trocaDeMensagens() {
	contexto, cancel := context.WithCancel(context.Background())
	defer cancel()
	entradaChan := make(chan string)  // Canal para capturar a entrada do usuário
	c.iniciarEntradaUsuario(entradaChan)

	// Inicia as goroutines para monitoramento e recebimento de mensagens
	go c.iniciarMovimentacao()
	//go c.monitorarBateria(contexto)
	go c.receberMensagem()

	for {
		if !c.estaProcessando(){// Exibe o menu apenas quando o usuário pode interagir
			fmt.Println("Posição X: ", c.CoordenadaX)
			fmt.Println("Posição Y: ", c.CoordenadaY)
			fmt.Printf("🔋 Bateria: %d%%\n", c.Bateria)
			fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("          🚀 MENU PRINCIPAL 🚀        ")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("  1️⃣  | Solicitar Pontos de Recarga")
			fmt.Println("  2️⃣  | Encerrar Conexão")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Print(" 👉 Escolha uma opção: ")
		}
		select {
		case <-contexto.Done():
			return

		case msg := <-c.msgChan:
			limparTela()
			log.Println("[Troca] Mensagem recebida:", msg.Tipo)
			c.processarMensagens(msg, entradaChan) // 👈 Passa canal
		case opcao := <-entradaChan:
			log.Println("Entrou")
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

