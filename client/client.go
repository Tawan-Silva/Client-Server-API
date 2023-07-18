package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type Quote struct {
	Bid float64 `json:"bid,string"`
}

func main() {
	ctx := context.Background()

	ctx, cancel := context.WithTimeout(ctx, time.Millisecond*300)
	defer cancel()

	quote, err := getQuoteFromServer(ctx)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Valor atual do câmbio: %.2f\n", quote.Bid)

	// Salvando a cotação em um arquivo "cotacao.txt"
	saveQuoteToFile(quote)
}

func getQuoteFromServer(ctx context.Context) (*Quote, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:8080/cotacao", nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %v", err)
	}

	client := http.Client{
		Timeout: time.Millisecond * 300,
	}

	res, err := client.Do(req)
	if err != nil {
		// Verifica se o erro foi causado pelo timeout do contexto
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("Timeout do contexto atingido ao fazer a requisição HTTP para obter a cotação")
		} else {
			log.Printf("Erro ao fazer a requisição HTTP para obter a cotação: %v\n", err)
		}
		return nil, fmt.Errorf("erro ao fazer a requisição HTTP para obter a cotação: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		return nil, fmt.Errorf("erro na resposta HTTP: %s, Message: %v", res.Status, string(body))
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o corpo da resposta: %v", err)
	}

	var quote Quote
	err = json.Unmarshal(body, &quote)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar a resposta do servidor: %v", err)
	}

	return &quote, nil
}

func saveQuoteToFile(quote *Quote) {
	file, err := os.Create("cotacao.txt")
	if err != nil {
		log.Println("Erro ao criar o arquivo cotacao.txt:", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("Dólar: %.2f\n", quote.Bid))
	if err != nil {
		log.Println("Erro ao escrever a cotação no arquivo:", err)
		return
	}

	log.Println("Cotação salva no arquivo cotacao.txt")
}
