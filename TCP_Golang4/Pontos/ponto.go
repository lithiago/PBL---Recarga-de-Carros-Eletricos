package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

// Constantes
const (
	MaxFila          = 10
	TempoRecargaBase = 60 * time.Second
	VelocidadeMedia  = 13.8
	EarthRadius      = 6371000
)

// Structs
type Ponto struct {
	conn            net.Conn
	CoordenadaX        float64
	CoordenadaY       float64
	fila            []Carro
	disponibilidade bool
	mu              sync.Mutex
	msgCanal        chan Mensagem
	envioChan       chan Mensagem
	Id              int
}

type Carro struct {
	CoordenadaX  float64 `json:"coordenadaX"`
	CoordenadaY float64 `json:"coordenadaY"`
	Bateria   int     `json:"bateria"`
	Id        int     `json:"id"`
	Liberacao bool	`json:"liberacao"`
}

type DadosParaCarro struct {
	Distancia   float64 `json:"distancia"`
	TempoEspera float64 `json:"tempo_espera"`
	Fila        []Carro `json:"fila"`
	CoordenadaX    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

type Coordenadas struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Mensagem struct {
	Tipo           string `json:"tipo"`
	Conteudo       []byte `json:"conteudo"`
	OrigemMensagem string `json:"origemmensagem"`
}

// Inicialização
func NewPosto(host, port string) (*Ponto, error) {
	address := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao servidor: %v", err)
	}
	type initMsg struct{
		CoordenadaX    float64 `json:"CoordenadaX"`
		CoordenadaY   float64 `json:"coordenadaY"`
	}
	
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	minX, maxX := 0.0, 5000.0 // de 0 a 10 km no eixo X
	minY, maxY := 0.0, 5000.0 // de 0 a 10 km no eixo Y
	ponto := &Ponto{
		conn:            conn,
		CoordenadaX:        randomInRange(r, minX, maxX),
		CoordenadaY:       randomInRange(r, minY, maxY),
		disponibilidade: true,
		fila:            make([]Carro, 0),
		msgCanal:        make(chan Mensagem, 100),
		envioChan:       make(chan Mensagem, 100),
	}


	msgBytes, err := json.Marshal(initMsg{CoordenadaX: ponto.CoordenadaX, CoordenadaY: ponto.CoordenadaY})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("erro ao serializar mensagem inicial: %v", err)
	}

	mensagem := Mensagem{
		Tipo:           "Conexao",
		Conteudo:       msgBytes,
		OrigemMensagem: "PONTO",
	}
	

	// Disparar envio dedicado
	go ponto.gerenciarEnvio()

	ponto.envioChan <- mensagem

	return ponto, nil
}
// Função para gerar um número aleatório dentro de um intervalo
func randomInRange(r *rand.Rand, min, max float64) float64 {
	return min + r.Float64()*(max-min)
}
// Escrita segura no servidor
func (p *Ponto) gerenciarEnvio() {
	for msg := range p.envioChan {
		if err := sendJSON(p.conn, msg); err != nil {
			log.Println("Erro ao enviar mensagem:", err)
		}
	}
}

func sendJSON(conn net.Conn, msg Mensagem) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %w", err)
	}
	_, err = conn.Write(append(data, '\n'))
	return err
}

// Leitura de dados do servidor
func (p *Ponto) receberDados() {
	reader := bufio.NewReader(p.conn)
	for {
		dados, err := reader.ReadBytes('\n')
		if err != nil {
			log.Println("Erro ao ler resposta:", err)
			break
		}

		var msg Mensagem
		if err := json.Unmarshal(dados, &msg); err != nil {
			log.Printf("Erro ao decodificar JSON: %v\nConteúdo bruto: %s\n", err, string(dados))
			continue
		}
		p.msgCanal <- msg
	}
}

// Reserva
func (p *Ponto) processarReserva(msg Mensagem) error {
	log.Println("Processando Reserva")

	type dados struct {
		MsgString string `json:"msgString"`
		CarroId   int    `json:"carroId"`
		PontoId int `json:"pontoId"`
	}

	var carro Carro
	if err := json.Unmarshal(msg.Conteudo, &carro); err != nil {
		return fmt.Errorf("erro ao decodificar carro: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.fila = append(p.fila, carro)

	sucessoMsg, _ := json.Marshal(dados{MsgString: "Reserva Confirmada", CarroId: carro.Id, PontoId: p.Id})
	p.envioChan <- Mensagem{
		Tipo:           "RESERVACONFIRMADA",
		Conteudo:       sucessoMsg,
		OrigemMensagem: "PONTO",
	}

	return nil
}

func (p *Ponto) processarFilaRecarga() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.fila) == 0 {
		return
	}

	for i, carro := range p.fila {
		type dadosPosicao struct {
			CarroId        int  `json:"carroId"`
			PosicaoFila    int  `json:"posicaoFila"`
			PontoId        int  `json:"pontoId"`
			Liberacao      bool `json:"liberacao"`
			Disponibilidade bool `json:"disponibilidade"`
			Autorizado     bool `json:"autorizado"`
		}

		autorizado := false
		if i == 0 && carro.Liberacao && p.disponibilidade {
			autorizado = true
			p.disponibilidade = false // travar ponto para recarga
		}

		msgPosicao, err := json.Marshal(dadosPosicao{
			CarroId:        carro.Id,
			PosicaoFila:    i + 1,
			PontoId:        p.Id,
			Liberacao:      carro.Liberacao,
			Disponibilidade: p.disponibilidade,
			Autorizado:     autorizado,
		})
		if err != nil {
			log.Printf("Erro ao serializar posição para carro %d: %v\n", carro.Id, err)
			continue
		}

		p.envioChan <- Mensagem{
			Tipo:           "AtualizacaoPosicaoFila",
			Conteudo:       msgPosicao,
			OrigemMensagem: "PONTO",
		}
	}
}




// Processador principal
func (p *Ponto) trocaDeMensagens() {
	go p.receberDados()

	for {
		msg := <-p.msgCanal
		
		for i := range p.fila{
			log.Printf("Carros na fila [%d]\n", p.fila[i].Id)
		}
		switch msg.Tipo {
		case "RESERVA":
			log.Println("[PONTO] Recebi Solicitação")
			if err := p.processarReserva(msg); err != nil {
				log.Println("Erro em Reserva:", err)
			}
		case "ID":
			type dadosID struct {
				IdCliente int `json:"idCliente"`
			}
			var idPonto dadosID
			log.Printf("Conteúdo bruto da mensagem recebida: %s", string(msg.Conteudo))
			if err := json.Unmarshal(msg.Conteudo, &idPonto); err != nil {
				log.Println("Erro:", err)
	
			}
			p.Id = idPonto.IdCliente
			log.Printf("✅ ID recebido e atribuído: %d\n", p.Id)
		case "Recarga":
			type bateria struct {
				Bateria  int `json:"bateria"`
				CarroId  int `json:"carroId"`
				PontoId  int `json:"pontoId"`
				TotalCarregado int `json:"totalCarregado"`
			}
		
			var bateriaResp bateria
			if err := json.Unmarshal(msg.Conteudo, &bateriaResp); err != nil {
				log.Println("Erro ao decodificar JSON:", err)
				return
			}
		
			if bateriaResp.Bateria < 100 {
				log.Printf("🔋 Carro [%d] com %d%% de carga, já carregou %d%% total a ser pago: R$ %.2f", bateriaResp.CarroId, bateriaResp.Bateria, bateriaResp.TotalCarregado, float64(bateriaResp.TotalCarregado) * 10)
				p.disponibilidade = false
			} else {
				log.Printf("✅ Carro [%d] completou a recarga, seu saldo já será registrado. Liberando ponto...", bateriaResp.CarroId)
				p.disponibilidade = true
				p.processarFilaRecarga()

			}
		case "CustosDoCarro":
			type bateria struct{
				Bateria int `json:"bateria"`
				CarroId      int `json:"carroId"`
				PontoId int `json:"pontoId"`
				TotalCarregado int `json:"totalCarregado"`		
			}
			var bateriaResp bateria
			if err := json.Unmarshal(msg.Conteudo, &bateriaResp); err != nil {
				log.Println("Erro ao decodificar JSON:", err)
				return
			}

			type Pagamento struct {
				CarroId int `json:"carroId"`
				PontoId int `json:"pontoId"`
				Custo float64 `json:"custo"`
				CoordenadaX float64 `json:"coordenadaX"`
				CoordenadaY float64 `json:"coordenadaY"`
			}
			if len(p.fila) == 0 {
				log.Printf("✅ Fila do ponto [%d] está vazia após liberação", p.Id)
			} else {
				log.Printf("📍 Próximo carro na fila do ponto [%d]: %d", p.Id, p.fila[0].Id)
				p.fila = p.fila[1:]
				p.disponibilidade = true
				p.processarFilaRecarga()
			}
			conteudoJSON, _ := json.Marshal(Pagamento{CarroId: bateriaResp.CarroId, PontoId: p.Id, Custo: float64(bateriaResp.TotalCarregado) * 10, CoordenadaX: p.CoordenadaX, CoordenadaY: p.CoordenadaY})
			p.envioChan <- Mensagem{Tipo: "CustosDoCarro", Conteudo: conteudoJSON, OrigemMensagem: "PONTO"}
		case "Liberacao":
			type info struct {
				CarroId     int `json:"carroId"`
				Liberacao bool `json:"liberacao"`
				PontoId int `json:"pontoId"`
			}
			var dados info
			if err := json.Unmarshal(msg.Conteudo, &dados); err != nil {
				log.Println("Erro ao decodificar pontos:", err)
				return
			}

			for i := range p.fila{
				if p.fila[i].Id == dados.CarroId{
					p.fila[i].Liberacao = dados.Liberacao
				}
			}
			p.processarFilaRecarga()
		default:
			log.Printf("Recebi mensagem com Tipo: '%s', Origem: '%s', Conteúdo bruto: %s", msg.Conteudo, msg.OrigemMensagem, string(msg.Conteudo))
		}
	}
}

// Finalizador
func (p *Ponto) Close() {
	log.Println("Encerrando conexão do ponto de recarga...")

	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			log.Println("Erro ao fechar conexão:", err)
		} else {
			log.Println("Conexão fechada com sucesso.")
		}
	}
	close(p.envioChan)
}

// Main
func main() {
	ponto, err := NewPosto("server", "3000")
	if err != nil {
		log.Fatalln("Erro ao criar ponto:", err)
	}
	defer ponto.Close()

	ponto.trocaDeMensagens()
}
