package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"sort"
	"log"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
)

type Server struct {
	listenAddr string        
	ln         net.Listener  
	quitch     chan struct{} 
	msgch      chan []byte   
	clients    map[int]Cliente 
	pontos     map[int]net.Conn
	mu         sync.Mutex
}

type Ponto struct {
	Conn          net.Conn   `json:"conn"`
	Latitude      float64    `json:"latitude"`
	Longitude     float64    `json:"longitude"`
	Fila          []string   `json:"fila"`
	TempoDeEspera int       `json:"tempoDeEspera"`
}

type Coordenadas struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Cliente struct {
	Endereco           net.Conn             `json:"endereco"`
	SaldoDevedor       map[string]float64   `json:"saldoDevedor"`
	ExtratoDePagamento map[string]float64   `json:"extratoDePagamento"`
}

// Struct para organizar o envio de dados
type ReqPontoDeRecarga struct{
	Latitude float64
	Longitude float64
	Bateria int
}
func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		quitch:     make(chan struct{}),
		msgch:      make(chan []byte),
		clients:    make(map[int]Cliente),
		pontos:     make(map[int]net.Conn),
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

	go s.acceptLoop()
	go s.handleMessages()

	<-s.quitch
	return nil
}

func (s *Server) receber_mensagem(conn net.Conn) (string, error) {
	reader := bufio.NewReader(conn)
	mensagem, err := reader.ReadString('\n')
	fmt.Println("Error:", err)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(mensagem), nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			continue
		}
		msg, err := s.receber_mensagem(conn)
		if err != nil {
			fmt.Println("Erro ao receber mensagem:", err)
			conn.Close()
			continue
		}

		if msg == "client" {
			fmt.Println("Entrou!!!!")
			s.mu.Lock()
			id := len(s.clients)
			cliente := Cliente{
				Endereco:          conn,
				SaldoDevedor:      make(map[string]float64),
				ExtratoDePagamento: make(map[string]float64),
			}
			s.clients[id] = cliente
			s.mu.Unlock()
			fmt.Println("Novo cliente conectado:", conn.RemoteAddr().String())
		} else if msg == "ponto" {
			fmt.Println("Entrou:", msg)
			s.mu.Lock()
			id := len(s.pontos)
			s.pontos[id] = conn
			s.mu.Unlock()
			fmt.Println("Novo ponto conectado:", conn.RemoteAddr().String())
		}

		go s.readLoop(conn)
	}
}

func contains(lista []string, item string) bool {
	for _, v := range lista {
		if v == item {
			return true	
		}
	}
	return false
}

func (s *Server) readLoop(conn net.Conn) {
	defer func() {
		fmt.Println("Cliente desconectado:", conn.RemoteAddr())
		conn.Close()
	}()

	for {	
		mens_receb, err := s.receber_mensagem(conn)
		if err != nil {
			fmt.Println("Erro ao receber mensagem:", err)
			return
		}

		fmt.Println(mens_receb)

		partes := strings.Fields(mens_receb)

		if len(partes) == 0 {
			conn.Write([]byte("Comando inválido\n"))
			continue
		}

		if contains(partes, "Carro") {
			switch partes[0] {
			case "Pontos":
				if len(partes) != 5 {  // Corrigido para 5 (Pontos, lat, long, bateria)
					conn.Write([]byte("ERRO: Formato deve ser 'Pontos latitude longitude bateria'\n"))
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
	
				s.processaSolicitacaoPontos(conn, bateria, latitude, longitude)
			case "Sair":
				fmt.Println("Cliente solicitou desconexão:", conn.RemoteAddr())
				return
			default:
				conn.Write([]byte("Comando não reconhecido\n"))
			}
		}

		if contains(partes, "Posto") {
			fmt.Println("Olá posto!")
		}
	}
}

func (s *Server) buscaPontosDeRecarga(bateria int, latitude float64, longitude float64) ([]Ponto, error) {
	// Estrutura para a requisição ao Ponto de Recarga nela contém os dados que o ponto precisa.
	req := ReqPontoDeRecarga{
		Bateria:  bateria,
		Latitude: latitude,
		Longitude: longitude,
	}

	// Serializa a mensagem para JSON
	mensagem, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar mensagem: %v", err)
	}
	
	buffer := make([]byte, 4096)  // Buffer para leitura da resposta
	var pontos []Ponto  // Lista de pontos que serão retornados

	s.mu.Lock()  // Protege o acesso ao mapa de conexões
	defer s.mu.Unlock()

	// Envia a mensagem de "Localização" para cada ponto de recarga
	for id, conn := range s.pontos {
		// Enviar a mensagem de reserva
		_, err := conn.Write([]byte("Localizacao\n"))
		if err != nil {
			fmt.Printf("Erro ao enviar mensagem de reserva para ponto %d: %v\n", id, err)
			continue
		}
		// Enviar a mensagem serializada com os dados de bateria, latitude e longitude
		_, err = conn.Write(append(mensagem, '\n'))  // Adiciona nova linha para indicar fim da mensagem
		if err != nil {
			fmt.Printf("Erro ao enviar dados para ponto %d: %v\n", id, err)
			continue
		}

		// Lê a resposta do ponto
		dados, err := conn.Read(buffer)
		if err != nil {
			fmt.Printf("Erro ao ler resposta do ponto %d: %v\n", id, err)
			continue
		}

		var ponto Ponto
		resposta := string(buffer[:dados])
		err = json.Unmarshal([]byte(resposta), &ponto)
		if err != nil {
			fmt.Printf("Erro ao decodificar resposta do ponto %d: %v\n", id, err)
			continue
		}

		// Associa a conexão ao ponto e adiciona à lista
		ponto.Conn = conn
		pontos = append(pontos, ponto)
	}

	// Se não houver pontos disponíveis
	if len(pontos) == 0 {
		return nil, fmt.Errorf("nenhum ponto disponível")
	}

	// Retorna os pontos encontrados
	return pontos, nil
}



func sortPontosByDistance(pontos []Ponto, latVeiculo, lonVeiculo float64) []Ponto {
	posVeiculo := Coordenadas{Latitude: latVeiculo, Longitude: lonVeiculo}
	
	sort.SliceStable(pontos, func(i, j int) bool {
		posI := Coordenadas{Latitude: pontos[i].Latitude, Longitude: pontos[i].Longitude}
		posJ := Coordenadas{Latitude: pontos[j].Latitude, Longitude: pontos[j].Longitude}
		return distanciaEntrePontos(&posVeiculo, &posI) < distanciaEntrePontos(&posVeiculo, &posJ)
	})
	
	return pontos
}

func (s *Server) handleMessages() {
	for msg := range s.msgch {
		fmt.Println("Mensagem processada:", string(msg))
		fmt.Println("Enviando resposta ao cliente:", "Mensagem recebida: "+string(msg))
	}
}

 func (s *Server) processaSolicitacaoPontos(conn net.Conn, bateria int, latitude float64, longitude float64) {
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

func main() {
	server := NewServer(":3000")
	log.Fatal(server.Start())
}