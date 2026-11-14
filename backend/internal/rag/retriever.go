package rag

import (
	"context"
	"fmt"
	"math"

	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
)

type ScoredChunk struct {
	Page          *int    `json:"page,omitempty"`
	LocationLabel string  `json:"locationLabel,omitempty"`
	SectionIndex  int     `json:"-"`
	ChunkID       string  `json:"chunkId"`
	SourceID      string  `json:"sourceId"`
	SourceTitle   string  `json:"sourceTitle"`
	Content       string  `json:"content"`
	ChunkIndex    int     `json:"chunkIndex"`
	Score         float64 `json:"score"`
}

type Retriever struct{ DB *gorm.DB }

func NewRetriever(database *gorm.DB) *Retriever { return &Retriever{DB: database} }

// Search 仅检索当前用户在该笔记本选中的就绪来源；空选择直接返回空数组。
func (r *Retriever) Search(ctx context.Context, userID, notebookID string, sourceIDs []string, queryVec []float32, topK int) ([]ScoredChunk, error) {
	items := make([]ScoredChunk, 0)
	if len(sourceIDs) == 0 {
		return items, nil
	}
	var norm float64
	for _, value := range queryVec {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("查询向量包含无效数值")
		}
		norm += float64(value) * float64(value)
	}
	if len(queryVec) == 0 || norm == 0 {
		return nil, fmt.Errorf("查询向量不能为空或零向量")
	}
	if topK <= 0 {
		topK = 6
	}
	topK = min(topK, 100)
	vector := pgvector.NewVector(queryVec)
	err := r.DB.WithContext(ctx).Raw(`SELECT c.id AS chunk_id, c.source_id, s.title AS source_title,
		c.content, c.chunk_index,c.page,c.location_label,c.section_index, 1 - (c.embedding <=> ?::vector) AS score
		FROM chunks c JOIN sources s ON s.id = c.source_id AND s.user_id = c.user_id AND s.notebook_id = c.notebook_id
		WHERE c.user_id = ? AND c.notebook_id = ? AND s.user_id = ? AND s.notebook_id = ?
		AND s.status = 'ready' AND s.id IN ?
		ORDER BY c.embedding <=> ?::vector, c.id LIMIT ?`,
		vector, userID, notebookID, userID, notebookID, sourceIDs, vector, topK).Scan(&items).Error
	return items, err
}
