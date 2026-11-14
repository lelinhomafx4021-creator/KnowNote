package rag

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"knownote/internal/db"
	"knownote/internal/llm"
	"knownote/internal/model"
)

func TestRetrieverIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://knownote:knownote@localhost:5432/knownote?sslmode=disable"
	}
	database, err := db.Open(ctx, dsn)
	if err != nil {
		t.Skip("DATABASE_URL 不可用，跳过向量检索集成测试")
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Migrate(ctx, database, 1536); err != nil {
		t.Fatal(err)
	}
	owner := model.User{Email: "t2-rag-" + uuid.NewString() + "@example.com", PasswordHash: "测试", Nickname: "检索测试"}
	other := model.User{Email: "t2-rag-" + uuid.NewString() + "@example.com", PasswordHash: "测试", Nickname: "其他用户"}
	for _, user := range []*model.User{&owner, &other} {
		if err := database.Create(user).Error; err != nil {
			t.Fatal(err)
		}
		id := user.ID
		t.Cleanup(func() { database.Where("id = ?", id).Delete(&model.User{}) })
	}
	notebook := model.Notebook{UserID: owner.ID, Title: "检索测试", Emoji: "📘"}
	second := model.Notebook{UserID: owner.ID, Title: "其他笔记本", Emoji: "📘"}
	foreign := model.Notebook{UserID: other.ID, Title: "其他用户笔记本", Emoji: "📘"}
	for _, item := range []*model.Notebook{&notebook, &second, &foreign} {
		if err := database.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	client := llm.NewFake(1536)
	texts := []string{"apple orchard fruit harvest", "quantum physics particle energy", "apple orchard fruit harvest", "apple orchard fruit harvest", "apple orchard fruit harvest"}
	vectors, err := client.Embed(ctx, texts)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i, text := range texts {
		book := notebook
		status := "ready"
		if i == 2 {
			status = "processing"
		}
		if i == 3 {
			book = second
		}
		if i == 4 {
			book = foreign
		}
		source := model.Source{NotebookOwner: model.NotebookOwner{UserID: book.UserID, NotebookID: book.ID}, Title: "来源标题", Type: "text", Status: status}
		if err := database.Create(&source).Error; err != nil {
			t.Fatal(err)
		}
		chunk := model.Chunk{NotebookOwner: source.NotebookOwner, SourceID: source.ID, Content: text, Embedding: model.NewVector(vectors[i])}
		if err := database.Create(&chunk).Error; err != nil {
			t.Fatal(err)
		}
		ids = append(ids, source.ID)
	}
	r := NewRetriever(database)
	items, err := r.Search(ctx, owner.ID, notebook.ID, ids, vectors[0], 6)
	if err != nil || len(items) != 2 {
		t.Fatalf("用户、笔记本与就绪状态过滤失败：%v，共 %d 项", err, len(items))
	}
	if items[0].SourceID != ids[0] || items[0].Score < 0.999 || items[0].Score < items[1].Score || items[0].SourceTitle != "来源标题" || items[0].Content != texts[0] {
		t.Fatal("余弦排序或结果字段错误")
	}
	items, err = r.Search(ctx, owner.ID, notebook.ID, ids[:1], vectors[0], 1)
	if err != nil || len(items) != 1 || items[0].SourceID != ids[0] {
		t.Fatal("来源选择或 topK 未生效")
	}
	items, err = r.Search(ctx, owner.ID, notebook.ID, nil, nil, 0)
	if err != nil || len(items) != 0 || items == nil {
		t.Fatal("空来源选择应该返回空数组")
	}
	if _, err := r.Search(ctx, owner.ID, notebook.ID, ids, make([]float32, 1536), 6); err == nil {
		t.Fatal("零查询向量应报错")
	}
}
