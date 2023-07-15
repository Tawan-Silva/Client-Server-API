package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Quote struct {
	Code       string  `json:"code"`
	Codein     string  `json:"codein"`
	Name       string  `json:"name"`
	High       float64 `json:"high,string"`
	Low        float64 `json:"low,string"`
	VarBid     float64 `json:"varBid,string"`
	PctChange  float64 `json:"pctChange,string"`
	Bid        float64 `json:"bid,string"`
	Ask        float64 `json:"ask,string"`
	Timestamp  string  `json:"timestamp"`
	CreateDate string  `json:"create_date"`
}

type QuoteResponse struct {
	USDBRL Quote `json:"USDBRL"`
}

func main() {
	db, err := sql.Open("mysql", "root:12345678@tcp(localhost:3306)/currency")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Handler para o endpoint /cotacao
	http.HandleFunc("/cotacao", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
		defer cancel()

		quote, err := getUSDQuote(ctx)
		if err != nil {
			http.Error(w, "Erro ao obter a cotação do dólar", http.StatusInternalServerError)
			return
		}

		err = insertQuote(ctx, db, quote)
		if err != nil {
			http.Error(w, "Erro ao salvar a cotação no banco de dados", http.StatusInternalServerError)
			return
		}

		// Salvando a cotação em um arquivo "cotacao.txt"
		saveQuoteToFile(quote)

		// Retornando a cotação como resposta
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(quote)
	})

	// Iniciando o servidor na porta 8080
	log.Println("Servidor iniciado na porta 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getUSDQuote(ctx context.Context) (*Quote, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://economia.awesomeapi.com.br/json/last/USD-BRL", nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %v", err)
	}

	client := http.Client{
		Timeout: time.Millisecond * 200,
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

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("erro ao ler o corpo da resposta: %v", err)
	}

	var response QuoteResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("erro ao decodificar a resposta do servidor: %v", err)
	}

	return &response.USDBRL, nil
}

func insertQuote(ctx context.Context, db *sql.DB, quote *Quote) error {
	stmt, err := db.Prepare("INSERT INTO quotes (code, codein, name, high, low, varBid, pctChange, bid, ask, timestamp, create_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("erro ao preparar a declaração SQL: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx,
		quote.Code,
		quote.Codein,
		quote.Name,
		quote.High,
		quote.Low,
		quote.VarBid,
		quote.PctChange,
		quote.Bid,
		quote.Ask,
		quote.Timestamp,
		quote.CreateDate,
	)
	if err != nil {
		// Verifica se o erro foi causado pelo timeout do contexto
		if ctx.Err() == context.DeadlineExceeded {
			log.Println("Timeout do contexto atingido ao inserir a cotação no banco de dados")
		} else {
			log.Printf("Erro ao inserir a cotação no banco de dados: %v\n", err)
		}
		return fmt.Errorf("erro ao inserir a cotação no banco de dados: %v", err)
	}

	return nil
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
