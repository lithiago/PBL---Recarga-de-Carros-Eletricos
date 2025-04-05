package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sort"
	"sync"
	"time"
)

type Server struct {
	listenAddr string
	ln         net.Listener
	quitch     chan struct{}
	msgch      chan Mensagem
	clients    map[int]*Cliente
	pontos     map[int]*Ponto
	mu         sync.RWMutex
	clientID   int // Contador seguro para IDs únicos
	pontoID    int
}

/* type Ponto struct {
	Conn          net.Conn
	Latitude      float64
	Longitude     float64
	Fila          []string
	TempoDeEspera int
	mu            sync.Mutex
} */

type Carro struct {
	Latitude  float64
	Longitude float64
	conn net.Conn
	bateria int
}

type Coordenadas struct {
	Latitude  float64
	Longitude float64
}

type Ponto struct {
	Conn net.Conn `json:"conn"`
	Distancia   float64 `json:"distancia"`
	TempoEspera float64 `json:"tempo_espera"`
	Fila        []Carro     `json:"fila"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	mu            sync.Mutex

}

type Cliente struct {
	Endereco           net.Conn
	SaldoDevedor       map[string]float64
	ExtratoDePagamento map[string]float64
}

type ReqPontoDeRecarga struct {
	Latitude  float64
	Longitude float64
	conn net.Conn
}

type Mensagem struct {
	Tipo           string `json:"tipo"`
	Conteudo       []byte `json:"conteudo"`
	OrigemMensagem string `json:"origemmensagem"`
}

func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		quitch:     make(chan struct{}),
		msgch:      make(chan Mensagem, 100),
		clients:    make(map[int]*Cliente),
		pontos:     make(map[int]*Ponto),
		clientID:   0,
		pontoID:    0,
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

	go s.acceptLoop(ctx)
	<-s.quitch
	return nil
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			reader := bufio.NewReader(conn)
			msgBytes, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Erro ao ler mensagem inicial:", err)
				return
			}

			var mensagem Mensagem
			if err := json.Unmarshal([]byte(msgBytes), &mensagem); err != nil {
				fmt.Println("Erro ao decodificar JSON inicial:", err)
				return
			}

			s.mu.Lock()
			if mensagem.OrigemMensagem == "CARRO" {
				id := s.clientID
				s.clients[id] = &Cliente{
					Endereco:           conn,
					SaldoDevedor:       make(map[string]float64),
					ExtratoDePagamento: make(map[string]float64),
				}
				s.clientID++
				fmt.Println("Novo cliente (CARRO) conectado:", conn.RemoteAddr())
			} else if mensagem.OrigemMensagem == "PONTO" {
				id := s.pontoID
				ponto := &Ponto{
					Conn:          conn,
					Fila:          make([]Carro, 0),
					TempoEspera: 0.0,
				}
				s.pontos[id] = ponto
				s.pontoID++
				go ponto.processarFila()
				fmt.Println("Novo ponto conectado:", conn.RemoteAddr())
			}
			s.mu.Unlock()

			s.readLoop(conn)
		}(conn)
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
		tempoEspera := p.TempoEspera
		p.mu.Unlock()

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

func (s *Server) readLoop(conn net.Conn) {
	reader := bufio.NewReader(conn)

	for {
		mensagemJSON, err := reader.ReadBytes('\n')
		if err != nil {
			log.Println("Erro ao ler mensagem:", err)
			break
		}

		var msg Mensagem
		err = json.Unmarshal(mensagemJSON, &msg)
		if err != nil {
			log.Println("Erro ao decodificar JSON:", err)
			continue
		}

		s.processarMensagens(msg, conn)
	}
}

func (s *Server) processarMensagens(msg Mensagem, conn net.Conn) {
	if msg.OrigemMensagem == "CARRO" || msg.OrigemMensagem == "PONTO" && msg.Tipo == "Pontos"  {
		var coordenadas Coordenadas
		if err := json.Unmarshal(msg.Conteudo, &coordenadas); err != nil {
			log.Println("Erro ao decodificar pontos:", err)
			return
		}

		pontos, err := s.buscaPontosDeRecarga(coordenadas.Latitude, coordenadas.Longitude, conn)
		if err != nil {
			log.Println("Erro ao buscar pontos:", err)
			return
		}

		log.Println("[DEBUG] Dados recebidos:", pontos)

		resp := struct {
			Listapontos []*Ponto `json:"listapontos"`
		}{
			Listapontos: pontos,
		}

		conteudoJSON, err := json.Marshal(resp)
		if err != nil {
			log.Println("Erro ao serializar resposta:", err)
			return
		}

		enviarMensagem(conn, Mensagem{
			Tipo:           "PONTOS",
			Conteudo:       conteudoJSON,
			OrigemMensagem: "Servidor",
		})
	}
	if msg.OrigemMensagem == "PONTO" && msg.Tipo == "LocalizacaoResposta"{

	}
}

func enviarMensagem(conn net.Conn, msg Mensagem) {
	dados, err := json.Marshal(msg)
	if err != nil {
		log.Println("Erro ao serializar mensagem:", err)
		return
	}
	dados = append(dados, '\n')
	conn.Write(dados)
}




func (s *Server) buscaPontosDeRecarga(latitude, longitude float64, conn net.Conn) ([]*Ponto, error) {
	req := ReqPontoDeRecarga{
		Latitude:  latitude,
		Longitude: longitude,
		conn: conn,
	}

	conteudoJSON, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	pontos := make([]*Ponto, 0) // Alterado para armazenar ponteiros

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

			enviarMensagem(p.Conn, Mensagem{
				Tipo:           "Localizacao",
				Conteudo:       conteudoJSON,
				OrigemMensagem: "Servidor",
			})

			

			select {

	
			case msg := <-s.msgch:
				fmt.Println(msg.Tipo)
				pontoJSON := msg.Conteudo
				var pontoResp Ponto
				if err := json.Unmarshal(pontoJSON, &pontoResp); err != nil {
					fmt.Println("erro ao decodificar carro: %w", err)
				}
				mu.Lock()
				pontos = append(pontos, &pontoResp) // Agora armazenamos um ponteiro
				mu.Unlock()
			}
			/* buffer := make([]byte, 4096)
			dados, err := p.Conn.Read(buffer)
			if err != nil {
				return
			} */

			/* var msg Mensagem
			if err := json.Unmarshal(buffer[:dados], &msg); err != nil {
				return
			} */

			
		}(ponto)
	}

	wg.Wait()

	if len(pontos) == 0 {
		return nil, fmt.Errorf("nenhum ponto disponível")
	}

	// Ordenação corrigida para trabalhar com ponteiros
	sort.Slice(pontos, func(i, j int) bool {
		return pontos[i].TempoEspera < pontos[j].TempoEspera
	})

	return pontos, nil
}



func main() {
	server := NewServer(":3000")
	log.Fatal(server.Start())
}
