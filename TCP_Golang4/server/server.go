package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"sort"
	"sync"
	"time"
)
const (
	MaxFila          = 10
	TempoRecargaBase = 60 * time.Second
	VelocidadeMedia  = 13.8 // m/s (50 km/h)
	EarthRadius      = 6371000
)


type Server struct {
	listenAddr string
	ln         net.Listener
	quitch     chan struct{}
	msgch      chan Mensagem
	clients    map[int]*Carro
	pontos     map[int]*Ponto
	mu         sync.RWMutex
	clientID   int // Contador seguro para IDs únicos
	pontoID    int
}

type Reserva struct {
	PontoId int `json:"ponto"`
	CoordenadaX float64 `json:"coordenadaX"`
	CoordenadaY float64 `json:"coordenadaY"`
	Bateria   int	`json:"bateria"`
	CarroId int `json:"id"`
}
type Carro struct{
	Conn net.Conn
	CoordenadaX float64 `json:"coordenadaX"`
	CoordenadaY float64 `json:"coordenadaY"`
	Bateria   int	`json:"bateria"`
	Id int `json:"id"`
}

type Coordenadas struct {
	CoordenadaX  float64 
	CoordenadaY float64
}

type Ponto struct {
	Conn net.Conn 
	Distancia   float64 `json:"distancia"`
	TempoEspera float64 `json:"tempo_espera"`
	Fila        []Carro     `json:"fila"`
	CoordenadaX    float64 `json:"CoordenadaX"`
	CoordenadaY   float64 `json:"coordenadaY"`
	Id int `json:"id"`
	mu            sync.Mutex

}


type ReqPontoDeRecarga struct {
	CoordenadaX  float64
	CoordenadaY float64
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
		clients:    make(map[int]*Carro),
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

			var id int
			type dadosID struct {
				IdCliente int `json:"idCliente"`
			}
			s.mu.Lock()
			if mensagem.OrigemMensagem == "CARRO" {
				id = s.clientID
				s.clients[id] = &Carro{
					Conn: conn,
					Id:   id,
				}
				s.clientID++
				fmt.Println("Novo cliente (CARRO) conectado:", conn.RemoteAddr())
			} else if mensagem.OrigemMensagem == "PONTO" {
				var posicoes Coordenadas
				_ = json.Unmarshal(mensagem.Conteudo, &posicoes)
				id = s.pontoID
				s.pontos[id] = &Ponto{
					Conn:        conn,
					Fila:        make([]Carro, 0),
					TempoEspera: 0.0,
					Id:          id,
					CoordenadaX: posicoes.CoordenadaX,
					CoordenadaY: posicoes.CoordenadaY,
				}
				s.pontoID++
				fmt.Println("Novo ponto conectado:", conn.RemoteAddr())
				fmt.Println("ID Ponto:", s.pontoID)
			}			
			dadosCliente := dadosID{IdCliente: id} 
			// Envia o ID atribuído ao cliente de volta
			// Envia o ID atribuído ao cliente de volta
			conteudoJSON, err := json.Marshal(dadosCliente)
			if err != nil {
				fmt.Println("Erro ao codificar JSON de ID:", err)
				return
			}
			enviarMensagem(conn, Mensagem{Tipo: "ID", Conteudo: conteudoJSON, OrigemMensagem: "SERVIDOR"})
			s.mu.Unlock()
			s.readLoop(conn)
		}(conn)
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
	switch msg.OrigemMensagem{
	case "CARRO":
		switch msg.Tipo{
			case "Pontos":
				var coordenadas Coordenadas
				if err := json.Unmarshal(msg.Conteudo, &coordenadas); err != nil {
					log.Println("Erro ao decodificar pontos:", err)
					return
				}
				s.mu.RLock()
				var pontos []*Ponto
				for id, _ := range s.pontos {
					tempo, dist := s.calcularTempoEspera(coordenadas, s.pontos[id])
					s.pontos[id].TempoEspera = tempo
					s.pontos[id].Distancia = dist
					pontos = append(pontos, s.pontos[id])
				}
				s.mu.RUnlock()
				sort.Slice(pontos, func(i, j int) bool {
					return pontos[i].TempoEspera < pontos[j].TempoEspera
				})
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
			case "RESERVA":
				log.Println("Entrou no Caso de Reserva")

				var reserva Reserva
				if err := json.Unmarshal(msg.Conteudo, &reserva); err != nil {
					log.Println("Erro ao decodificar pontos:", err)
					return
				}
				ponto := s.pontos[reserva.PontoId]
				conteudoJSON, err := json.Marshal(Carro{CoordenadaX: reserva.CoordenadaX, CoordenadaY: reserva.CoordenadaY, Bateria: reserva.Bateria, Id: reserva.CarroId})
				if err != nil {
					log.Println("Erro ao serializar resposta:", err)
					return
				}
				enviarMensagem(ponto.Conn, Mensagem{Tipo: "RESERVA", Conteudo: conteudoJSON, OrigemMensagem: "SERVIDOR"})
			case "LiberarPonto":
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

				enviarMensagem(s.pontos[dados.PontoId].Conn, Mensagem{Tipo: "Liberacao", Conteudo: msg.Conteudo, OrigemMensagem: "SERVIDOR"} )


			case "Recarga":
				type bateria struct{
					Bateria int `json:"bateria"`
					CarroId      int `json:"carroId"`
					PontoId int `json:"pontoId"`		
				}
				var bateriaAtt bateria
				if err := json.Unmarshal(msg.Conteudo, &bateriaAtt); err != nil {
					log.Println("Erro ao decodificar pontos:", err)
					return
				}
				conteudoJSON, err := json.Marshal(bateria{Bateria: bateriaAtt.Bateria, CarroId: bateriaAtt.CarroId, PontoId: bateriaAtt.PontoId})
				if err != nil {
					log.Println("Erro ao serializar resposta:", err)
					return
				}
				ponto := s.pontos[bateriaAtt.PontoId]
				enviarMensagem(ponto.Conn, Mensagem{Tipo: "Recarga", Conteudo: conteudoJSON, OrigemMensagem: "SERVIDOR"})
			case "RecargaConcluida":
				type bateria struct{
					Bateria int `json:"bateria"`
					CarroId      int `json:"carroId"`
					PontoId int `json:"pontoId"`
					TotalCarregado int `json:"totalCarregado"`		
				}
				var bateriaCarregada bateria
				if err := json.Unmarshal(msg.Conteudo, &bateriaCarregada); err != nil {
					log.Println("Erro ao decodificar pontos:", err)
					return
				}
				conteudoJSON, err := json.Marshal(bateria{Bateria: bateriaCarregada.Bateria, CarroId: bateriaCarregada.CarroId, PontoId: bateriaCarregada.PontoId, TotalCarregado: bateriaCarregada.TotalCarregado})
				if err != nil {
					log.Println("Erro ao serializar resposta:", err)
					return
				}
				ponto := s.pontos[bateriaCarregada.PontoId]
				enviarMensagem(ponto.Conn, Mensagem{Tipo: "CustosDoCarro", Conteudo: conteudoJSON, OrigemMensagem: "SERVIDOR"})
			}
	case "PONTO":
		switch msg.Tipo{
			case "RESERVACONFIRMADA":

				type dados struct {
					MsgString string `json:"msgString"`
					CarroId   int    `json:"carroId"`
					PontoId int `json:"pontoId"`
				}
				var infos dados
				if err := json.Unmarshal(msg.Conteudo, &infos); err != nil {
					log.Println("Erro ao decodificar pontos:", err)
					return
				}
				carro := s.clients[infos.CarroId]
				ponto := s.pontos[infos.PontoId]

				type dadosReserva struct{
					CoordenadaX float64 `json:"coordenadaX"`
					CoordenadaY float64 `json:"coordenadaY"`
					PontoId int `json:"pontoId"`
				}

				conteudoJSON, err := json.Marshal(dadosReserva{CoordenadaX: ponto.CoordenadaX, CoordenadaY: ponto.CoordenadaY, PontoId: ponto.Id})
				if err != nil {
					log.Println("Erro ao serializar resposta:", err)
					return
				}
				log.Println("Entrou em Reserva: ", carro.Conn.RemoteAddr())

				enviarMensagem(carro.Conn, Mensagem{Tipo: "RESERVA", Conteudo:conteudoJSON, OrigemMensagem: "SERVIDOR"})
			case "AtualizacaoPosicaoFila":
				type dadosPosicao struct {
					CarroId     int `json:"carroId"`
					PosicaoFila int `json:"posicaoFila"`
					PontoId     int `json:"pontoId"`
					Liberacao bool `json:"liberacao"`
				}

				var dados dadosPosicao
				if err := json.Unmarshal(msg.Conteudo, &dados); err != nil {
					log.Println("Erro ao decodificar pontos:", err)
					return
				}

				type resposta struct{
					CarroId     int `json:"carroId"`
					PosicaoFila int `json:"posicaoFila"`
					PontoId     int `json:"pontoId"`
				}
/* 
				resp := resposta{CarroId: dados.CarroId, PosicaoFila: dados.PosicaoFila, PontoId: dados.PontoId}
				conteudoJSON, _ := json.Marshal(resp) */
				carro := s.clients[dados.CarroId]
				enviarMensagem(carro.Conn, Mensagem{Tipo: "AtualizacaoPosicaoFila", Conteudo: msg.Conteudo, OrigemMensagem: "SERVIDOR"})
			
			case "CustosDoCarro":
				type Pagamento struct {
					CarroId     int     `json:"carroId"`
					PontoId     int     `json:"pontoId"`
					Custo       float64 `json:"custo"`
					CoordenadaX float64 `json:"coordenadaX"`
					CoordenadaY float64 `json:"coordenadaY"`
				}
			
				var dadosPagamento Pagamento
				if err := json.Unmarshal(msg.Conteudo, &dadosPagamento); err != nil {
					log.Println("Erro ao decodificar pontos:", err)
					return
				}
			
				// Registrar em JSON (acrescentando no arquivo)
				func(p Pagamento) {
					const fileName = "pagamentos.json"
			
					// Lê pagamentos existentes, se houver
					var pagamentos []Pagamento
					if fileData, err := os.ReadFile(fileName); err == nil {
						_ = json.Unmarshal(fileData, &pagamentos)
					}
			
					// Adiciona novo pagamento
					pagamentos = append(pagamentos, p)
			
					// Salva de volta no arquivo
					fileData, err := json.MarshalIndent(pagamentos, "", "  ")
					if err != nil {
						log.Println("Erro ao gerar JSON dos pagamentos:", err)
						return
					}
					if err := os.WriteFile(fileName, fileData, 0644); err != nil {
						log.Println("Erro ao salvar arquivo de pagamentos:", err)
					}
				}(dadosPagamento)
			
				// Continua a lógica existente
				enviarMensagem(s.clients[dadosPagamento.CarroId].Conn, Mensagem{Tipo: "Pagamento", Conteudo: msg.Conteudo, OrigemMensagem: "SERVIDOR"})
			

		}
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
	log.Println("Servidor enviou os dados para o carro")
}



func (s *Server) calcularTempoEspera(coordCarro Coordenadas, ponto *Ponto) (tempoTotal, distancia float64) {
	distancia = s.distanciaPara(coordCarro, Coordenadas{
		CoordenadaX:  ponto.CoordenadaX,  // já é Y em metros
		CoordenadaY: ponto.CoordenadaY, // já é X em metros
	})
	log.Println("distancia: ", distancia)

	tempoViagem := distancia / VelocidadeMedia
	tempoEspera := float64(len(ponto.Fila))

	return tempoViagem + tempoEspera, distancia
}


func (s *Server) distanciaPara(coordCarro, coordPonto Coordenadas) float64 {
	dx := coordCarro.CoordenadaX - coordPonto.CoordenadaX
	dy := coordCarro.CoordenadaY - coordPonto.CoordenadaY
	return math.Hypot(dx, dy)
}



func main() {
	server := NewServer(":3000")
	log.Fatal(server.Start())
}
