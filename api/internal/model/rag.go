package model

import "time"

type RAGChunk struct {
	ID       int64
	Document string
	DocType  string
	Pages    string
	Section  string
	Content  string
	Score    float64
}

type RAGDocument struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	DocType    string    `json:"doc_type"`
	SourceURL  string    `json:"source_url"`
	IngestedAt time.Time `json:"ingested_at"`
	ChunkCount int       `json:"chunk_count"`
}
