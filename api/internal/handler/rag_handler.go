package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"taxpod-crm/api/internal/service"
)

type RAGHandler struct {
	rag         *service.RAGService
	defaultTopK int
}

func NewRAGHandler(rag *service.RAGService, defaultTopK int) *RAGHandler {
	return &RAGHandler{rag: rag, defaultTopK: defaultTopK}
}

type askRequest struct {
	Question string  `json:"question"`
	DocType  *string `json:"doc_type"`
	TopK     int     `json:"top_k"`
}

func (h *RAGHandler) Ask(c *gin.Context) {
	var request askRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Request body must be valid JSON.")
		return
	}
	request.Question = strings.TrimSpace(request.Question)
	if request.Question == "" || len([]rune(request.Question)) > 1000 {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "question must contain between 1 and 1000 characters.")
		return
	}
	if request.DocType != nil {
		value := strings.TrimSpace(*request.DocType)
		if value == "" {
			request.DocType = nil
		} else {
			valid := map[string]bool{"e-invoice": true, "sst": true, "income-tax": true, "other": true}
			if !valid[value] {
				writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "doc_type must be e-invoice, sst, income-tax, or other.")
				return
			}
			request.DocType = &value
		}
	}
	if request.TopK == 0 {
		request.TopK = h.defaultTopK
	}
	if request.TopK < 1 || request.TopK > 10 {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "top_k must be between 1 and 10.")
		return
	}

	result, err := h.rag.Ask(c.Request.Context(), request.Question, request.DocType, request.TopK)
	if err != nil {
		status, code := http.StatusBadGateway, "UPSTREAM_ERROR"
		if errors.Is(err, context.DeadlineExceeded) {
			status, code = http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT"
		}
		writeError(c, status, code, "The tax assistant is temporarily unavailable. Please try again.")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *RAGHandler) Documents(c *gin.Context) {
	documents, err := h.rag.ListDocuments(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL", "Could not load the document list.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": documents})
}

func writeError(c *gin.Context, status int, code, message string) {
	requestID, _ := c.Get("request_id")
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message, "request_id": requestID}})
}
