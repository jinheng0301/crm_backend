package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"taxpod-crm/api/internal/repository"
	"taxpod-crm/api/pkg/gemini"
)

type question struct {
	ID             string   `json:"id"`
	Question       string   `json:"question"`
	DocType        string   `json:"doc_type"`
	ExpectedTitles []string `json:"expected_titles"`
	Answerable     bool     `json:"answerable"`
}

type result struct {
	ID             string   `json:"id"`
	Question       string   `json:"question"`
	ExpectedTitles []string `json:"expected_titles"`
	Retrieved      []string `json:"retrieved_titles"`
	Hit            bool     `json:"hit"`
	Abstained      bool     `json:"abstained,omitempty"`
}

func main() {
	setPath := flag.String("set", filepath.Join("..", "docs", "eval", "golden.json"), "evaluation question set JSON")
	flag.Parse()
	_ = godotenv.Load("../.env", ".env")
	apiKey, databaseURL := os.Getenv("GEMINI_API_KEY"), os.Getenv("DATABASE_URL")
	if apiKey == "" || databaseURL == "" {
		fatal("GEMINI_API_KEY and DATABASE_URL are required")
	}
	modelName := env("EMBEDDING_MODEL", "gemini-embedding-001")
	dimensions, err := gemini.ParseDimensions(env("EMBEDDING_DIM", "768"))
	if err != nil || dimensions != 768 {
		fatal("EMBEDDING_DIM must match the current 768-dimensional schema")
	}
	threshold := 0.55
	if _, err := fmt.Sscan(env("RAG_MIN_SCORE", "0.55"), &threshold); err != nil || threshold < -1 || threshold > 1 {
		fatal("RAG_MIN_SCORE must be between -1 and 1")
	}
	topK := 8
	if _, err := fmt.Sscan(env("RAG_CANDIDATES", "8"), &topK); err != nil || topK < 1 || topK > 20 {
		fatal("RAG_CANDIDATES must be between 1 and 20")
	}
	data, err := os.ReadFile(*setPath)
	if err != nil {
		fatal("read evaluation set: " + err.Error())
	}
	var questions []question
	if err := json.Unmarshal(data, &questions); err != nil {
		fatal("parse evaluation set: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatal("configure database: " + err.Error())
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("connect to database: " + err.Error())
	}
	repo := repository.NewRAGRepository(pool)
	embedder := gemini.New(apiKey, env("LLM_MODEL", "gemini-2.5-flash"), modelName, dimensions)
	var results []result
	hits, answerable, abstained, unanswerable := 0, 0, 0, 0
	for _, item := range questions {
		queryCtx, queryCancel := context.WithTimeout(ctx, 10*time.Second)
		vector, err := embedder.EmbedQuery(queryCtx, item.Question)
		queryCancel()
		if err != nil {
			fatal(item.ID + " embedding failed: " + err.Error())
		}
		var docType *string
		if item.DocType != "" {
			docType = &item.DocType
		}
		chunks, err := repo.Search(ctx, vector, docType, topK)
		if err != nil {
			fatal(item.ID + " retrieval failed: " + err.Error())
		}
		row := result{ID: item.ID, Question: item.Question, ExpectedTitles: item.ExpectedTitles, Retrieved: []string{}}
		for _, chunk := range chunks {
			row.Retrieved = append(row.Retrieved, chunk.Document)
			if item.Answerable {
				for _, expected := range item.ExpectedTitles {
					if strings.Contains(strings.ToLower(chunk.Document), strings.ToLower(expected)) {
						row.Hit = true
					}
				}
			}
		}
		if item.Answerable {
			answerable++
			if row.Hit {
				hits++
			}
		} else {
			unanswerable++
			row.Abstained = len(chunks) == 0 || chunks[0].Score < threshold
			if row.Abstained {
				abstained++
			}
		}
		results = append(results, row)
	}
	hitRate := 0.0
	if answerable > 0 {
		hitRate = float64(hits) / float64(answerable)
	}
	refusalRate := 0.0
	if unanswerable > 0 {
		refusalRate = float64(abstained) / float64(unanswerable)
	}
	report := map[string]any{
		"hit_at_k": hitRate, "hits": hits, "answerable_questions": answerable,
		"refusal_rate": refusalRate, "abstained": abstained, "unanswerable_questions": unanswerable,
		"top_k": topK, "minimum_score": threshold, "results": results,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fatal(err.Error())
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
