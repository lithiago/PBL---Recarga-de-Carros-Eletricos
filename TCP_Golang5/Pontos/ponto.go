package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"
)
const (
	MaxFila          = 10
	TempoRecargaBase = 60 * time.Second
	VelocidadeMedia  = 13.8 // m/s (50 km/h)
	EarthRadius      = 6371000
)


type Ponto struct {
	conn            net.Conn
	latitude        float64
	longitude       float64
	fila            []Carro
	disponibilidade bool
	mu           sync.Mutex
	wg sync.WaitGroup
	quit chan struct{}
}

type Carro struct {
	conn        net.Conn
	Bateria     int
	Latitude    float64
	Longitude   float64
	PosicaoFila int
}

type Coordenadas struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Formato de mensagem para retornar ao servidor
type Mensagem struct {
	Conteudo interface{}
	Tipo     string
}

func NewPosto(host string, port string) (*Ponto, error) {
	address := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", address)
	fmt.Fprintln(conn, "ponto")
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

// Essa função recebe os dados e não desserializa para string. Mantém o formato JSON
func (c *Ponto) receberDados() ([]byte, error) {
	buffer := make([]byte, 4096)
	dados, err := c.conn.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler resposta do servidor: %v", err)
	}
	return buffer[:dados], nil
}
func (c *Ponto) JSONDadosDeVeiculos(dados []byte) (Carro, error) {
	var dadosJSON Carro
	err := json.Unmarshal(dados, &dadosJSON)
	if err != nil {
		return dadosJSON, fmt.Errorf("erro ao decodificar JSON: %v", err)
	}
	return dadosJSON, nil

}

// Reserva

func (p *Ponto) processarReserva() error {
	data, err := p.receberDados()
	if err != nil {
		return err
	}

	var carro Carro
	if err := json.Unmarshal(data, &carro); err != nil {
		return fmt.Errorf("erro ao decodificar carro: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.fila) >= MaxFila {
		return p.enviarMensagem(Mensagem{
			Tipo:    "ReservaRecusada",
			Conteudo: "Fila cheia",
		})
	}

	p.fila = append(p.fila, carro)
	return p.enviarMensagem(Mensagem{
		Tipo:    "ReservaConfirmada",
		Conteudo: len(p.fila) - 1,
	})
}

func serializarMensagem(m Mensagem) []byte {
	dados, err := json.Marshal(m)
	if err != nil {
		fmt.Println("Erro ao serializar mensagem:", err)
	}
	return dados
}

func (c *Ponto) iniciarRecarga(cliente Carro) {
	for i := cliente.Bateria; i < 100; i++ {
		time.Sleep(1 * time.Second)
		cliente.Bateria = i + 1
	}
	mensagemStruct := Mensagem{Tipo: "Recarga", Conteudo: cliente}
	mensagem := serializarMensagem(mensagemStruct)
	_, err := c.conn.Write(append(mensagem, '\n'))
	if err != nil {
		fmt.Println("Erro ao enviar mensagem:", err)
	}
	c.mu.Lock()
	c.fila = c.fila[1:]
	c.mu.Unlock()
}


func (p *Ponto) processarLocalizacao() error {
	data, err := p.receberDados()
	if err != nil {
		return err
	}

	var carro Carro
	if err := json.Unmarshal(data, &carro); err != nil {
		return fmt.Errorf("erro ao decodificar carro: %w", err)
	}

	tempo, distancia := p.calcularTempoEspera(carro.Latitude, carro.Longitude)
	resposta := map[string]interface{}{
		"distancia":    distancia,
		"tempo_espera": tempo,
		"fila":         len(p.fila),
		"disponivel":   p.disponibilidade,
		"latitude": p.latitude,
		"longitude": p.longitude,
	}

	return p.enviarMensagem(Mensagem{
		Tipo:     "LocalizacaoResposta",
		Conteudo: resposta,
	})
}


func (p *Ponto) enviarMensagem(msg Mensagem) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("erro ao serializar mensagem: %w", err)
	}

	if _, err := p.conn.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("erro ao enviar mensagem: %w", err)
	}
	return nil
}

func (p *Ponto) calcularTempoEspera(lat, lon float64) (float64, float64) {
	coords := Coordenadas{lat, lon}
	distancia := p.distanciaPara(coords)
	tempoViagem := distancia / VelocidadeMedia
	tempoTotal := tempoViagem + float64(len(p.fila))*TempoRecargaBase.Seconds()
	return tempoTotal, distancia
}

func (p *Ponto) distanciaPara(pos Coordenadas) float64 {
	lat1 := p.latitude * math.Pi / 180
	lon1 := p.longitude * math.Pi / 180
	lat2 := pos.Latitude * math.Pi / 180
	lon2 := pos.Longitude * math.Pi / 180

	dLat := lat2 - lat1
	dLon := lon2 - lon1

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadius * c
}


func (c *Ponto) trocaDeMensagens() {
	defer c.wg.Done()

	for {
		select {
		case <-c.quit:
			return
		default:
			msgType, err := c.receberSolicitacao()
			if err != nil {
				log.Printf("Erro ao receber mensagem: %v", err)
				continue
			}

			switch msgType {
			case "Localizacao":
				if err := c.processarLocalizacao(); err != nil {
					log.Printf("Erro ao processar localização: %v", err)
				}
			case "Reserva":
				if err := c.processarReserva(); err != nil {
					log.Printf("Erro ao processar reserva: %v", err)
				}
			default:
				log.Printf("Tipo de mensagem desconhecido: %s", msgType)
			}
		}
	}
}
func main() {
	// Criar um novo ponto
	ponto, err := NewPosto("server", "3000")
	if err != nil {
		fmt.Println("Erro ao criar ponto:", err)
		return
	}
	ponto.trocaDeMensagens()
}
