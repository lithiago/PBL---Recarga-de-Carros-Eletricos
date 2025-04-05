package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"
)

// Constantes de configuração
const (
	MaxFila          = 10
	TempoRecargaBase = 60 * time.Second
	VelocidadeMedia  = 13.8 // m/s (50 km/h)
	EarthRadius      = 6371000
)

// Structs (mantidas como requisitado)
type Ponto struct {
	conn            net.Conn
	latitude        float64
	longitude       float64
	fila            []Carro
	disponibilidade bool
	mu              sync.Mutex
	msgCanal        chan Mensagem
}

type Carro struct {
	Latitude  float64
	Longitude float64
	conn      net.Conn
	bateria   int
}

type DadosParaCarro struct {
	Conn        net.Conn
	Distancia   float64 `json:"distancia"`
	TempoEspera float64 `json:"tempo_espera"`
	Fila        []Carro `json:"fila"`
	Latitude    float64 `json:"latitude"`
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

// ==========================
// Inicialização do Ponto
// ==========================
func NewPosto(host, port string) (*Ponto, error) {
	address := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao servidor: %v", err)
	}

	initMsg := struct {
		Msg string `json:"msg"`
	}{"Inicio de Conexão"}

	msgBytes, err := json.Marshal(initMsg)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("erro ao serializar mensagem inicial: %v", err)
	}

	mensagem := Mensagem{
		Tipo:           "Conexao",
		Conteudo:       msgBytes,
		OrigemMensagem: "PONTO",
	}

	if err := sendJSON(conn, mensagem); err != nil {
		conn.Close()
		return nil, fmt.Errorf("erro ao enviar mensagem inicial: %v", err)
	}

	return &Ponto{
		conn:            conn,
		latitude:        -23.5505,
		longitude:       -46.6333,
		disponibilidade: true,
		fila:            make([]Carro, 0),
		msgCanal:        make(chan Mensagem),
	}, nil
}

// ==========================
// Envio de Mensagens
// ==========================
func (p *Ponto) enviarMensagem(msg Mensagem) error {
	return sendJSON(p.conn, msg)
}

func sendJSON(conn net.Conn, msg Mensagem) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %w", err)
	}
	_, err = conn.Write(append(data, '\n'))
	return err
}

// ==========================
// Comunicação com Servidor
// ==========================
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

		log.Printf("Mensagem recebida: %+v\n", msg)
		p.msgCanal <- msg
	}
}

// ==========================
// Processamento de Localização
// ==========================
func (p *Ponto) processarLocalizacao() error {
	msg := <-p.msgCanal

	var req Carro
	if err := json.Unmarshal(msg.Conteudo, &req); err != nil {
		return fmt.Errorf("erro ao decodificar carro: %w", err)
	}

	tempo, distancia := p.calcularTempoEspera(req.Latitude, req.Longitude)
	resposta := DadosParaCarro{
		Distancia:   distancia,
		TempoEspera: tempo,
		Fila:        p.fila,
		Latitude:    p.latitude,
		Longitude:   p.longitude,
	}

	conteudoJSON, err := json.Marshal(resposta)
	if err != nil {
		return fmt.Errorf("erro ao serializar resposta JSON: %w", err)
	}

	return p.enviarMensagem(Mensagem{
		Tipo:     "LocalizacaoResposta",
		Conteudo: conteudoJSON,
		OrigemMensagem: "PONTO",
	})
}

// ==========================
// Processamento de Reserva
// ==========================
func (p *Ponto) processarReserva() error {
	msg := <-p.msgCanal

	var carro Carro
	if err := json.Unmarshal(msg.Conteudo, &carro); err != nil {
		return fmt.Errorf("erro ao decodificar carro: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.fila) >= MaxFila {
		return p.enviarMensagem(Mensagem{
			Tipo:     "Falha",
			Conteudo: []byte(`{"msg":"Fila Cheia"}`),
		})
	}

	p.fila = append(p.fila, carro)

	return p.enviarMensagem(Mensagem{
		Tipo:     "ReservaConfirmada",
		Conteudo: []byte(`{"msg":"Reserva Confirmada"}`),
	})
}

// ==========================
// Processamento de Recarga
// ==========================
func (p *Ponto) iniciarRecarga(cliente *Carro) {
	for cliente.bateria < 100 {
		time.Sleep(1 * time.Second)
		cliente.bateria++
	}

	conteudoJSON, err := json.Marshal(cliente)
	if err != nil {
		log.Println("Erro ao serializar cliente:", err)
		return
	}

	p.enviarMensagem(Mensagem{Tipo: "Recarga", Conteudo: conteudoJSON})

	p.mu.Lock()
	if len(p.fila) > 0 {
		p.fila = p.fila[1:]
	}
	p.mu.Unlock()
}

// ==========================
// Cálculo de Distância e Tempo
// ==========================
func (p *Ponto) calcularTempoEspera(lat, lon float64) (float64, float64) {
	dist := p.distanciaPara(Coordenadas{Latitude: lat, Longitude: lon})
	viagem := dist / VelocidadeMedia
	espera := float64(len(p.fila)) * TempoRecargaBase.Seconds()
	return viagem + espera, dist
}

func (p *Ponto) distanciaPara(dest Coordenadas) float64 {
	toRad := func(deg float64) float64 { return deg * math.Pi / 180 }

	lat1, lon1 := toRad(p.latitude), toRad(p.longitude)
	lat2, lon2 := toRad(dest.Latitude), toRad(dest.Longitude)
	dLat := lat2 - lat1
	dLon := lon2 - lon1

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)

	return EarthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// ==========================
// Loop Principal
// ==========================
func (p *Ponto) trocaDeMensagens() {
	go p.receberDados()

	for {
		msg := <-p.msgCanal

		switch msg.Tipo {
		case "Localizacao":
			log.Printf("Processando: %s", msg.Tipo)
			if err := p.processarLocalizacao(); err != nil {
				log.Println("Erro em Localizacao:", err)
			}
		case "Reserva":
			if err := p.processarReserva(); err != nil {
				log.Println("Erro em Reserva:", err)
			}
		default:
			log.Printf("Tipo de mensagem desconhecido: %s", msg.Tipo)
		}
	}
}
func (p *Ponto) Close() {
	log.Println("Encerrando conexão do ponto de recarga...")

	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			log.Println("Erro ao fechar conexão:", err)
		} else {
			log.Println("Conexão fechada com sucesso.")
		}
	}

	// Se desejar fechar o canal também, descomente a linha abaixo:
	// close(p.msgCanal)
}

// ==========================
// Main
// ==========================
func main() {
	ponto, err := NewPosto("server", "3000")
	if err != nil {
		log.Fatalln("Erro ao criar ponto:", err)
	}
	defer ponto.Close()

	ponto.trocaDeMensagens()
}
