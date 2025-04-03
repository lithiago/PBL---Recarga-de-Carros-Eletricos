package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	listenAddr string
	ln         net.Listener
	quitch     chan struct{}
	msgch      chan []byte
	clients    map[int]Cliente
	pontos     map[int]*Ponto // Alterado para armazenar ponteiros
	mu         sync.RWMutex   // Usando RWMutex para leituras concorrentes
}

type Ponto struct {
	Conn          net.Conn
	Latitude      float64
	Longitude     float64
	Fila          []string
	TempoDeEspera int
	mu            sync.Mutex // Mutex específico para cada ponto
}

type Coordenadas struct {
	Latitude  float64
	Longitude float64
}

type Cliente struct {
	Endereco           net.Conn
	SaldoDevedor       map[string]float64
	ExtratoDePagamento map[string]float64
}

type ReqPontoDeRecarga struct {
	Latitude  float64
	Longitude float64
	Bateria   int
}

func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		quitch:     make(chan struct{}),
		msgch:      make(chan []byte, 100), // Buffer aumentado
		clients:    make(map[int]Cliente),
		pontos:     make(map[int]*Ponto),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	defer ln.Close()
	s.ln = ln

	fmt.Println("Servidor iniciado na porta", s.listenAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Inicia todas as goroutines
	go s.acceptLoop(ctx)
	go s.handleMessages(ctx)
	go s.monitorConexoes(ctx)

	<-s.quitch
	return nil
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := s.ln.Accept()
			if err != nil {
				continue
			}

			go func(conn net.Conn) {
				defer conn.Close()

				msg, err := s.receber_mensagem(conn)
				if err != nil {
					return
				}

				s.mu.Lock()
				if msg == "client" {
					id := len(s.clients)
					s.clients[id] = Cliente{
						Endereco:           conn,
						SaldoDevedor:       make(map[string]float64),
						ExtratoDePagamento: make(map[string]float64),
					}
					fmt.Println("Novo cliente conectado:", conn.RemoteAddr())
				} else if msg == "ponto" {
					id := len(s.pontos)
					ponto := &Ponto{
						Conn:          conn,
						Fila:          make([]string, 0),
						TempoDeEspera: 30, // Valor padrão
					}
					s.pontos[id] = ponto
					go ponto.processarFila() // Inicia goroutine para processar fila
					fmt.Println("Novo ponto conectado:", conn.RemoteAddr())
				}
				s.mu.Unlock()

				s.readLoop(ctx, conn)
			}(conn)
		}
	}
}

func (p *Ponto) processarFila() {
	for {
		p.mu.Lock()
		if len(p.Fila) == 0 {
			p.mu.Unlock()
			time.Sleep(1 * time.Second)
			continue
		}

		veiculo := p.Fila[0]
		tempoEspera := p.TempoDeEspera
		p.mu.Unlock()

		// Processa recarga
		time.Sleep(time.Duration(tempoEspera) * time.Second)

		p.mu.Lock()
		if len(p.Fila) > 0 && p.Fila[0] == veiculo {
			p.Fila = p.Fila[1:]
			mensagem := fmt.Sprintf("Recarga concluída para %s\n", veiculo)
			p.Conn.Write([]byte(mensagem))
		}
		p.mu.Unlock()
	}
}

func (s *Server) receber_mensagem(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)
	mensagem, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(mensagem), nil
}

func (s *Server) readLoop(ctx context.Context, conn net.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			mens_receb, err := s.receber_mensagem(conn)
			if err != nil {
				return
			}

			partes := strings.Fields(mens_receb)
			if len(partes) == 0 {
				conn.Write([]byte("Comando inválido\n"))
				continue
			}

			if strings.Contains(mens_receb, "Carro") {
				switch partes[0] {
				case "Pontos":
					if len(partes) != 5 {
						conn.Write([]byte("ERRO: Formato inválido\n"))
						continue
					}

					latitude, err := strconv.ParseFloat(partes[1], 64)
					if err != nil {
						conn.Write([]byte("Erro: Latitude inválida\n"))
						continue
					}

					longitude, err := strconv.ParseFloat(partes[2], 64)
					if err != nil {
						conn.Write([]byte("Erro: Longitude inválida\n"))
						continue
					}

					bateria, err := strconv.Atoi(strings.TrimSpace(partes[4]))
					if err != nil {
						conn.Write([]byte("Erro: Bateria inválida\n"))
						continue
					}

					go s.processaSolicitacaoPontos(conn, bateria, latitude, longitude)
				case "Sair":
					return
				}
			}

			if strings.Contains(mens_receb, "Posto") {
				s.mu.RLock()
				defer s.mu.RUnlock()
				s.msgch <- []byte(mens_receb)
			}
		}
	}
}

func (s *Server) buscaPontosDeRecarga(bateria int, latitude, longitude float64) ([]Ponto, error) {
	req := ReqPontoDeRecarga{
		Bateria:   bateria,
		Latitude:  latitude,
		Longitude: longitude,
	}

	mensagem, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	pontos := make([]Ponto, 0)

	s.mu.RLock()
	pontosConexoes := make([]*Ponto, 0, len(s.pontos))
	for _, ponto := range s.pontos {
		pontosConexoes = append(pontosConexoes, ponto)
	}
	s.mu.RUnlock()

	for _, ponto := range pontosConexoes {
		wg.Add(1)

		go func(p *Ponto) {
			defer wg.Done()

			p.mu.Lock()
			defer p.mu.Unlock()

			if _, err := p.Conn.Write([]byte("Localizacao\n")); err != nil {
				return
			}

			if _, err := p.Conn.Write(append(mensagem, '\n')); err != nil {
				return
			}

			buffer := make([]byte, 4096)
			dados, err := p.Conn.Read(buffer)
			if err != nil {
				return
			}

			var pontoResp Ponto
			if err := json.Unmarshal(buffer[:dados], &pontoResp); err != nil {
				return
			}

			pontoResp.Conn = p.Conn
			mu.Lock()
			pontos = append(pontos, pontoResp)
			mu.Unlock()
		}(ponto)
	}

	wg.Wait()

	if len(pontos) == 0 {
		return nil, fmt.Errorf("nenhum ponto disponível")
	}

	return pontos, nil
}

func (s *Server) processaSolicitacaoPontos(conn net.Conn, bateria int, latitude, longitude float64) {
	pontos, err := s.buscaPontosDeRecarga(bateria, latitude, longitude)
	if err != nil {
		conn.Write([]byte("ERRO: " + err.Error() + "\n"))
		return
	}

	pontosOrdenados := sortPontosByDistance(pontos, latitude, longitude)
	jsonPontos, err := json.Marshal(pontosOrdenados)
	if err != nil {
		conn.Write([]byte("Erro ao processar resposta\n"))
		return
	}

	conn.Write(append(jsonPontos, '\n'))
}

func sortPontosByDistance(pontos []Ponto, latVeiculo, lonVeiculo float64) []Ponto {
	posVeiculo := Coordenadas{Latitude: latVeiculo, Longitude: lonVeiculo}

	sort.SliceStable(pontos, func(i, j int) bool {
		posI := Coordenadas{Latitude: pontos[i].Latitude, Longitude: pontos[i].Longitude}
		posJ := Coordenadas{Latitude: pontos[j].Latitude, Longitude: pontos[j].Longitude}
		return distanciaEntrePontos(posVeiculo, posI) < distanciaEntrePontos(posVeiculo, posJ)
	})

	return pontos
}

func distanciaEntrePontos(a, b Coordenadas) float64 {
	// Implementação simplificada da distância euclidiana
	latDiff := a.Latitude - b.Latitude
	lonDiff := a.Longitude - b.Longitude
	return latDiff*latDiff + lonDiff*lonDiff
}

func (s *Server) handleMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-s.msgch:
			go func(m []byte) {
				fmt.Println("Mensagem processada:", string(m))
				// Lógica adicional de processamento aqui
			}(msg)
		}
	}
}

func (s *Server) monitorConexoes(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			// Verifica clientes
			for id, cliente := range s.clients {
				if _, err := cliente.Endereco.Write([]byte("PING\n")); err != nil {
					delete(s.clients, id)
					fmt.Println("Cliente desconectado:", id)
				}
			}
			// Verifica pontos
			for id, ponto := range s.pontos {
				if _, err := ponto.Conn.Write([]byte("PING\n")); err != nil {
					delete(s.pontos, id)
					fmt.Println("Ponto desconectado:", id)
				}
			}
			s.mu.Unlock()
		}
	}
}

func main() {
	server := NewServer(":3000")
	log.Fatal(server.Start())
}