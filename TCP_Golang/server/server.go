package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

type Server struct {
	listenAddr string        // Endereço IP e porta onde o servidor escutará conexões
	ln         net.Listener  // Listener do Socket TCP
	quitch     chan struct{} // Canal de sinalização para encerrar o servidor
	msgch      chan []byte   // Canal para comunicação de mensagens
}

func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		quitch:     make(chan struct{}),
		msgch:      make(chan []byte), // Inicializa o canal para mensagens
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr) // Cria um listener TCP na porta definida.
	if err != nil {
		return err
	}
	defer ln.Close()

	s.ln = ln
	fmt.Println("Servidor iniciado na porta", s.listenAddr)

	go s.acceptLoop() // Agora o servidor aceita conexões

	go s.handleMessages() // Goroutine para processar as mensagens

	<-s.quitch // Bloqueia a execução até que o canal quitch receba um sinal (indicando o encerramento do servidor)
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.ln.Accept() // Bloqueia até que um cliente se conecte
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			continue
		}

		fmt.Println("Novo cliente conectado:", conn.RemoteAddr())
		go s.readLoop(conn) // Cada conexão é processada em uma goroutine para permitir múltiplos clientes simultâneos.
	}
}

func (s *Server) readLoop(conn net.Conn) {
	defer func() {
		fmt.Println("Cliente desconectado:", conn.RemoteAddr())
		conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msg := scanner.Text()
		if msg != "" {
			fmt.Println("Mensagem recebida do cliente:", msg)
			// Resposta para o cliente
			_, err := fmt.Fprintf(conn, "Mensagem recebida: %s\n", msg)
			if err != nil {
				fmt.Println("Erro ao enviar resposta ao cliente:", err)
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Erro de leitura do cliente:", err)
	}
}

func (s *Server) handleMessages() {
	for msg := range s.msgch { // Lê mensagens do canal msgch
		fmt.Println("Mensagem processada:", string(msg))

		// Envia uma resposta de volta ao cliente
		// Aqui, o servidor envia uma confirmação após processar a mensagem
		// Assumindo que o servidor mantém a conexão com o cliente aberta
		fmt.Println("Enviando resposta ao cliente:", "Mensagem recebida: "+string(msg))
	}
}

func main() {
	server := NewServer(":3000") // Cria um servidor escutando na porta 3000
	log.Fatal(server.Start())
}
