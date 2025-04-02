package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"os/exec"
	"runtime"
)

type Ponto struct {
	conn            net.Conn
	latitude        float64
	longitude       float64
	fila            []string
	disponibilidade bool
}

type ReqPontoDeRecarga struct{
	Bateria  int
	Latitude float64
	Longitude  float64
}

type Coordenadas struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
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
func (c *Ponto) JSONDadosDeVeiculos(dados []byte)(ReqPontoDeRecarga, error){
	var dadosJSON ReqPontoDeRecarga
	err := json.Unmarshal(dados, &dadosJSON)
	if err != nil {
        return dadosJSON, fmt.Errorf("erro ao decodificar JSON: %v", err)
    }
	return dadosJSON, nil
	
}
func (ponto *Ponto) distanciaEntrePontos(posicaoVeiculo Coordenadas) float64 {
	const earthRadius = 6371000

	latVeiculo := posicaoVeiculo.Latitude * math.Pi / 180
	lonVeiculo := posicaoVeiculo.Longitude * math.Pi / 180
	latPosto := ponto.latitude * math.Pi / 180
	lonPosto := ponto.longitude * math.Pi / 180

	dLat := latPosto - latVeiculo
	dLon := lonPosto - lonVeiculo

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(latVeiculo)*math.Cos(latPosto)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func (c *Ponto) calcularTempoDeEspera(latCliente float64, longCliente float64, bateria int) (float64, float64){
	// O tempo de carga total padrão 1 minuto
	// Velocidade Méda - 50KM/H -> 13,8 m/s
	coordenadasCliente := Coordenadas{Latitude: latCliente, Longitude: longCliente}
	distancia := c.distanciaEntrePontos(coordenadasCliente)
	tempo := distancia / 13.8
	tempoMaximo := tempo + float64(len(c.fila))
	return tempoMaximo, distancia
}


// Indicar Disponibilidade e fila
func (c *Ponto) retornaPonto(latCliente float64, longCliente float64, bateria int) error {
	// Corrigir a função calcularTempoDeEspera para retornar valores
	tempoMaximo, distancia := c.calcularTempoDeEspera(latCliente, longCliente, bateria)

	// Definição da struct local para resposta
	type resposta struct {
		Distancia   float64 `json:"distancia"`
		TempoMax    float64 `json:"tempo_max"`
		TamanhoFila int     `json:"tamanho_fila"`
	}

	// Criando a resposta
	resp := resposta{
		Distancia:   distancia,
		TempoMax:    tempoMaximo,
		TamanhoFila: len(c.fila),
	}

	// Serializando para JSON
	jsonData, err := json.Marshal(resp)
	if err != nil {
		fmt.Println("Erro ao serializar JSON:", err)
		return err
	}

	// Enviar os dados como string
	err = c.Send(string(jsonData))
	if err != nil {
		return fmt.Errorf("erro ao enviar solicitação: %v", err)
	}

	return nil
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
func (c *Ponto) trocaDeMensagens(){
	for{
		resp, err := c.receberSolicitacao()
		if err != nil {
			fmt.Println("Erro:", err)
			return
		}
		// Primeiro ele deve receber a solicitação de localização. Depois ele deve receber os parâmetros do cliente. Ou talvez fique melhor se enviar todos os dados de uma vez?
		if resp == "Localizacao"{
			dadosSerializados, err := c.receberDados()
			if err != nil {
				fmt.Println("erro ao receber dados:", err)
			}
			dadosVeiculo, err := c.JSONDadosDeVeiculos(dadosSerializados)
			if err != nil {
				fmt.Println("erro ao receber dados:", err)
			}
			c.retornaPonto(dadosVeiculo.Latitude, dadosVeiculo.Longitude, dadosVeiculo.Bateria)
			continue
		}
		if resp == "Reserva"{
			
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