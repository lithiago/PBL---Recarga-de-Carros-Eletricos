package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"encoding/json"
	"io"
	"math"
)

type Server struct {
	listenAddr string        // Endereço IP e porta onde o servidor escutará conexões
	ln         net.Listener  // Listener do Socket TCP
	quitch     chan struct{} // Canal de sinalização para encerrar o servidor
	msgch      chan []byte   // Canal para comunicação de mensagens
}


type Ponto struct {
	latitude float64
	longitude float64
	fila []string
	disponibilidade bool
}

type Coodernadas struct{
	latitude float64
	longitude float64
}

// Eu vou precisar ficar montirando a bateria do carro? Se sim atribuir a bateria a struct.
type Cliente struct{
	// Endereco net.Addr
	SaldoDevedor map[string]float64 // Key - Data || Value - Preço da Recarga
	ExtratoDePagamento map[string]float64 // Key - Data || Value - Preço da Recargat64 // Aqui 
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

		// Converter o endereço remoto para string
		endereco := conn.RemoteAddr().String()

		// Carregar clientes já armazenados no JSON
		clientes := make(map[string]Cliente)
		data, err := os.ReadFile("Clients/ClientesConectados.json")
		if err == nil {
			_ = json.Unmarshal(data, &clientes) // Ignorar erro se o arquivo estiver vazio
		}

		// Adicionar o novo cliente sem perder os anteriores
		clientes[endereco] = Cliente{
			SaldoDevedor:        make(map[string]float64),
			ExtratoDePagamento:  make(map[string]float64),
		}

		// Converter o mapa atualizado para JSON
		clientesJSON, err := json.MarshalIndent(clientes, "", "  ")
		if err != nil {
			fmt.Println("Erro ao serializar clientes:", err)
			continue
		}

		// Gravar no arquivo JSON
		err = os.WriteFile("Clients/ClientesConectados.json", clientesJSON, 0644)
		if err != nil {
			fmt.Println("Erro ao escrever arquivo JSON:", err)
			continue
		}

		fmt.Println("Novo cliente conectado:", endereco)

		// Cada conexão é processada em uma goroutine para permitir múltiplos clientes simultâneos.
		go s.readLoop(conn)
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
// Só precisa ser executado uma vez para obter a localização dos pontos. Essa é a função que retorna a requisição do cliente. Ordem buscaPontosDeRecarga -> distanciaEntrePontos -> enviaMelhoresOpcoesDePontos
func (s *Server) buscaPontosDeRecarga() (map[string]Ponto, error) {
    // 1. Abre o arquivo (corrigido para usar 'defer' corretamente)
    jsonPontos, err := os.Open("../Pontos/Pontos.json")
    if err != nil {
        return map[string]Ponto{}, fmt.Errorf("falha ao abrir arquivo JSON: %w", err)
    }
    defer jsonPontos.Close() // Garante que o arquivo será fechado

    // 2. Lê o conteúdo (usando io.ReadAll em vez do depreciado ioutil.ReadAll)
    byteValueJson, err := io.ReadAll(jsonPontos)
    if err != nil {
        return map[string]Ponto{}, fmt.Errorf("falha ao ler arquivo JSON: %w", err)
    }

    // 3. Decodifica o JSON
    var pontoMap map[string]Ponto
    if err := json.Unmarshal(byteValueJson, &pontoMap); err != nil {
        return map[string]Ponto{}, fmt.Errorf("falha ao decodificar JSON: %w", err)
    }

    return pontoMap, nil
}


func distanciaEntrePontos(posicaoVeiculo *Coodernadas, posicaoPosto *Coodernadas) float64{
	var earthRadius float64 = 6371000 
	// Converter graus para radianos
    latVeiculo := posicaoVeiculo.latitude * math.Pi / 180
    lonVeiculo := posicaoVeiculo.longitude * math.Pi / 180
    latPosto := posicaoPosto.latitude * math.Pi / 180
    lonPosto := posicaoPosto.longitude * math.Pi / 180

    // Diferenças
    dLat := latPosto - latPosto
    dLon := lonPosto - lonVeiculo

    // Fórmula de Haversine
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
	math.Cos(latVeiculo)*math.Cos(latPosto)*
	math.Sin(dLon/2)*math.Sin(dLon/2)
   c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

    // Distância em metros
    distancia := earthRadius * c

	return distancia
}

// Ordenar e assim definir as melhores opções. No momento só considero as melhores opções a partir da distância, não levando em consideração o tempo de espera.
func (s *Server) enviaMelhoresOpcoesDePontos(latitudeVeiculo float64, longitudeVeiculo float64, mapPonto map[string]Ponto) map[string]float64{
	posicaoVeiculo := Coodernadas{latitude: latitudeVeiculo, longitude: longitudeVeiculo}
	var opcoesPontos map[string]float64
	for id, p :=  range mapPonto{
		posicaoPosto := Coodernadas{latitude: p.latitude, longitude: p.longitude}
		opcoesPontos[id] = distanciaEntrePontos(&posicaoVeiculo, &posicaoPosto)	
	}
	return opcoesPontos
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
