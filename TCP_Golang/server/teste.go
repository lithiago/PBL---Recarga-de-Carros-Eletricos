/* package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

type Ponto struct {
	Latitude        float32  `json:"latitude"`
	Longitude       float32  `json:"longitude"`
	Fila            []string `json:"fila"`
	Disponibilidade bool     `json:"disponibilidade"`
}

func main() {
	// 1. Abre o arquivo
	jsonPontos, err := os.Open("../Pontos/Pontos.json")
	if err != nil {
		log.Fatalf("falha ao abrir arquivo JSON: %v", err)
	}
	defer jsonPontos.Close()

	// 2. Lê o conteúdo
	byteValueJson, err := io.ReadAll(jsonPontos)
	if err != nil {
		log.Fatalf("falha ao ler arquivo JSON: %v", err)
	}

	// 3. Decodifica o JSON para um map
	var pontosMap map[string]Ponto
	if err := json.Unmarshal(byteValueJson, &pontosMap); err != nil {
		log.Fatalf("falha ao decodificar JSON: %v", err)
	}

	// 4. Imprime os pontos
	for nome, p := range pontosMap {
		fmt.Printf("%s:\n", nome)
		fmt.Printf("  Localização: (%f, %f)\n", p.Latitude, p.Longitude)
		fmt.Printf("  Fila: %v\n", p.Fila)
		fmt.Printf("  Disponível: %t\n", p.Disponibilidade)
		fmt.Println("-------------------")
	}
} */