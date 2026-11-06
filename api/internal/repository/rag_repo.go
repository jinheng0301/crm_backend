package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"taxpod-crm/api/internal/model"
)

type RAGRepository struct{ db *pgxpool.Pool }

func NewRAGRepository(db *pgxpool.Pool) *RAGRepository { return &RAGRepository{db: db} }

func (r *RAGRepository) Search(ctx context.Context, vector []float32, docType *string, limit int) ([]model.RAGChunk, error) {
	values := make([]string, len(vector))
	for i, value := range vector {
		values[i] = fmt.Sprintf("%.8f", value)
	}
	vectorParam := "[" + strings.Join(values, ",") + "]"
	rows, err := r.db.Query(ctx, `
		SELECT c.id, d.title, d.doc_type,
		       CASE WHEN c.page_start = 0 THEN ''
		            WHEN c.page_end = c.page_start THEN c.page_start::text
		            ELSE c.page_start::text || '-' || c.page_end::text END AS pages,
		       c.section, c.content, 1 - (c.embedding <=> $1::vector) AS score
		FROM chunks c JOIN documents d ON d.id = c.document_id
		WHERE ($2::text IS NULL OR d.doc_type = $2)
		ORDER BY c.embedding <=> $1::vector
		LIMIT $3`, vectorParam, docType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chunks []model.RAGChunk
	for rows.Next() {
		var chunk model.RAGChunk
		if err := rows.Scan(&chunk.ID, &chunk.Document, &chunk.DocType, &chunk.Pages, &chunk.Section, &chunk.Content, &chunk.Score); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (r *RAGRepository) Documents(ctx context.Context) ([]model.RAGDocument, error) {
	rows, err := r.db.Query(ctx, `
		SELECT d.id::text, d.title, d.doc_type, d.source_url, d.ingested_at, count(c.id)::int
		FROM documents d LEFT JOIN chunks c ON c.document_id = d.id
		GROUP BY d.id ORDER BY d.ingested_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var documents []model.RAGDocument
	for rows.Next() {
		var document model.RAGDocument
		if err := rows.Scan(&document.ID, &document.Title, &document.DocType, &document.SourceURL, &document.IngestedAt, &document.ChunkCount); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, rows.Err()
}
