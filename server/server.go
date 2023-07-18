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
	ID         int
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

type QuoteRepo struct {
	db *sql.DB
}

type QuoteRepository interface {
	InsertQuote(ctx context.Context, quote *Quote) error
}

func NewQuoteRepository(db *sql.DB) *QuoteRepo {
	return &QuoteRepo{db: db}
}

func (r *QuoteRepo) InsertQuote(ctx context.Context, quote *Quote) error {
	stmt, err := r.db.PrepareContext(ctx, "INSERT INTO quotes (code, codein, name, high, low, varBid, pctChange, bid, ask, timestamp, create_date) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
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
		return fmt.Errorf("erro ao inserir a cotação no banco de dados: %v", err)
	}

	return nil
}

type QuoteService struct {
	repo QuoteRepository
}

func main() {
	db, err := sql.Open("mysql", "root:12345678@tcp(localhost:3306)/")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = createCurrencyDB(db)
	if err != nil {
		log.Fatal(err)
	}

	db, err = sql.Open("mysql", "root:12345678@tcp(localhost:3306)/currency")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = createQuotesTable(db)
	if err != nil {
		log.Fatal(err)
	}

	repo := NewQuoteRepository(db)
	service := NewQuoteService(repo)

	http.HandleFunc("/cotacao", func(w http.ResponseWriter, r *http.Request) {
		HandleGetQuote(service, w, r)
	})

	log.Println("Servidor iniciado na porta 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func NewQuoteService(repo QuoteRepository) *QuoteService {
	return &QuoteService{repo: repo}
}

func (s *QuoteService) GetUSDQuote(ctx context.Context) (*Quote, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://economia.awesomeapi.com.br/json/last/USD-BRL", nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar requisição: %v", err)
	}

	client := http.Client{
		Timeout: time.Millisecond * 200,
	}
	res, err := client.Do(req)
	if err != nil {
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

func HandleGetQuote(service *QuoteService, w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	quote, err := service.GetUSDQuote(ctx)
	if err != nil {
		http.Error(w, "Erro ao obter a cotação do dólar", http.StatusInternalServerError)
		return
	}

	err = service.repo.InsertQuote(ctx, quote)
	if err != nil {
		http.Error(w, "Erro ao salvar a cotação no banco de dados", http.StatusInternalServerError)
		return
	}

	saveQuoteToFile(quote)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(quote)
}

func saveQuoteToFile(quote *Quote) {
	file, err := os.OpenFile("cotacao.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Println("Erro ao abrir o arquivo cotacao.txt:", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("Dólar: %.2f\n", quote.Bid))
	if err != nil {
		log.Println("Erro ao escrever a cotação no arquivo:", err)
		return
	}

	log.Println("Cotação adicionada ao arquivo cotacao.txt")
}

func createCurrencyDB(db *sql.DB) error {
	_, err := db.Exec("CREATE DATABASE IF NOT EXISTS currency")
	if err != nil {
		return fmt.Errorf("erro ao criar o banco de dados: %v", err)
	}
	return nil
}

func createQuotesTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS quotes (
		id INT AUTO_INCREMENT PRIMARY KEY,
		code VARCHAR(255) NOT NULL,
		codein VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		high DECIMAL(18, 2) NOT NULL,
		low DECIMAL(18, 2) NOT NULL,
		varBid DECIMAL(18, 2) NOT NULL,
		pctChange DECIMAL(18, 2) NOT NULL,
		bid DECIMAL(18, 2) NOT NULL,
		ask DECIMAL(18, 2) NOT NULL,
		timestamp VARCHAR(255) NOT NULL,
		create_date VARCHAR(255) NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("erro ao criar a tabela quotes: %v", err)
	}
	return nil
}
