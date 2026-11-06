package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"taxpod-crm/api/internal/handler"
	"taxpod-crm/api/internal/middleware"
	"taxpod-crm/api/internal/repository"
	"taxpod-crm/api/internal/service"
	"taxpod-crm/api/migrations"
	"taxpod-crm/api/pkg/gemini"
)

func main() {
	_ = godotenv.Load("../.env", ".env")
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	port := env("API_PORT", "18080")
	databaseURL := os.Getenv("DATABASE_URL")
	origins := splitCSV(env("CORS_ORIGINS", "http://localhost:15173"))
	geminiKey := os.Getenv("GEMINI_API_KEY")
	llmModel := env("LLM_MODEL", "gemini-2.5-flash")
	embeddingModel := env("EMBEDDING_MODEL", "gemini-embedding-001")
	dimensions, err := gemini.ParseDimensions(env("EMBEDDING_DIM", "768"))
	if err != nil {
		logger.Error("invalid embedding configuration", "error", err)
		os.Exit(1)
	}

	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	if geminiKey == "" || llmModel == "" || embeddingModel == "" {
		logger.Error("GEMINI_API_KEY, LLM_MODEL, and EMBEDDING_MODEL are required")
		os.Exit(1)
	}
	if dimensions != 768 {
		logger.Error("EMBEDDING_DIM must be 768 to match the current chunks.embedding schema", "configured", dimensions)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		logger.Error("configure database connection", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := pool.Ping(checkCtx); err != nil {
		cancel()
		logger.Error("database startup check failed", "error", err)
		os.Exit(1)
	}
	cancel()
	if err := migrations.Up(ctx, pool); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
	var mismatchedDocuments int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM documents WHERE embedding_model <> $1", embeddingModel).Scan(&mismatchedDocuments); err != nil {
		logger.Error("embedding model startup check failed", "error", err)
		os.Exit(1)
	}
	if mismatchedDocuments > 0 {
		logger.Error("stored chunks use a different embedding model", "configured_model", embeddingModel, "mismatched_documents", mismatchedDocuments)
		os.Exit(1)
	}

	gin.SetMode(env("GIN_MODE", gin.ReleaseMode))
	router := gin.New()
	router.SetTrustedProxies(nil)
	router.Use(requestID(), recoverer(logger), requestLogger(logger), bodyLimit(), cors(origins))
	health := func(c *gin.Context) {
		pingCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(pingCtx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "database": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "ok"})
	}
	router.GET("/healthz", health)
	api := router.Group("/api/v1")
	api.GET("/healthz", health)
	crmRepo := repository.NewCRMRepository(pool)
	crmService := service.NewCRMService(crmRepo)
	handler.NewCRMHandler(crmService).Register(api)
	minScore, err := strconv.ParseFloat(env("RAG_MIN_SCORE", "0.55"), 64)
	if err != nil || minScore < -1 || minScore > 1 {
		logger.Error("RAG_MIN_SCORE must be between -1 and 1")
		os.Exit(1)
	}
	candidates, err := strconv.Atoi(env("RAG_CANDIDATES", "8"))
	if err != nil || candidates < 1 || candidates > 20 {
		logger.Error("RAG_CANDIDATES must be between 1 and 20")
		os.Exit(1)
	}
	topK, err := strconv.Atoi(env("RAG_TOP_K", "6"))
	if err != nil || topK < 1 || topK > 10 || candidates < topK {
		logger.Error("RAG_TOP_K must be 1-10 and no greater than RAG_CANDIDATES")
		os.Exit(1)
	}
	geminiClient := gemini.New(geminiKey, llmModel, embeddingModel, dimensions)
	ragRepo := repository.NewRAGRepository(pool)
	ragService := service.NewRAGService(ragRepo, geminiClient, geminiClient, minScore, candidates)
	ragHandler := handler.NewRAGHandler(ragService, topK)
	ai := api.Group("/rag")
	ai.Use(middleware.NewAILimit(intEnv("AI_RATE_LIMIT_PER_MIN", 20)))
	ai.POST("/ask", ragHandler.Ask)
	api.GET("/rag/documents", ragHandler.Documents)

	server := &http.Server{Addr: ":" + port, Handler: router, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		logger.Info("API listening", "address", server.Addr, "cors_origins", strings.Join(origins, ","))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown failed", "error", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		logger.Info("http request", "request_id", c.GetString("request_id"), "method", c.Request.Method, "path", c.Request.URL.Path,
			"status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds())
	}
}

func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		var id [12]byte
		if _, err := rand.Read(id[:]); err != nil {
			c.Set("request_id", "unavailable")
		} else {
			value := hex.EncodeToString(id[:])
			c.Set("request_id", value)
			c.Header("X-Request-ID", value)
		}
		c.Next()
	}
}

func bodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	}
}

func recoverer(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("handler panic", "request_id", c.GetString("request_id"), "panic", recovered)
				if !c.Writer.Written() {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{
						"code": "INTERNAL", "message": "An unexpected error occurred.", "request_id": c.GetString("request_id"),
					}})
				}
			}
		}()
		c.Next()
	}
}

func cors(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
