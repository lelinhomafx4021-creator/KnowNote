package handler_test

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
	"io"
	"knownote/internal/db"
	"knownote/internal/handler"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/service/chat"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func t10API(t *testing.T) (*testAPI, *service.Service) {
	gin.SetMode(gin.TestMode)
	database := sourceDatabase(t)
	core := service.New(database, middleware.NewTokens(strings.Repeat("t", 32)), "fake", slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &testAPI{t: t, database: database, router: handler.NewRouter(core, []string{"http://localhost:5190"}, handler.ChatRoutes(core, chat.New(database, llm.NewFake(1536), core.Logger)))}, core
}
func TestT10TagsAndPermissions(t *testing.T) {
	a, _ := t10API(t)
	owner, other := a.register(), a.register()
	tag := decode[model.Tag](t, a.call("POST", "/api/tags", owner.Token, `{"name":" 学习 ","color":"#146c2e"}`, 200))
	if tag.Name != "学习" || tag.Color != "#146C2E" {
		t.Fatal("标签规范化失败")
	}
	a.call("POST", "/api/tags", owner.Token, `{"name":"学习"}`, 409)
	for _, body := range []string{`{}`, `{"name":" "}`, `{"name":"标签","color":"red"}`} {
		a.call("POST", "/api/tags", owner.Token, body, 400)
	}
	foreign := decode[model.Tag](t, a.call("POST", "/api/tags", other.Token, `{"name":"学习"}`, 200))
	for _, method := range []string{"PATCH", "DELETE"} {
		a.call(method, "/api/tags/"+tag.ID, other.Token, `{"name":"越权"}`, 404)
	}
	if len(decode[[]model.Tag](t, a.call("GET", "/api/tags", owner.Token, "", 200))) != 1 {
		t.Fatal("标签列表未隔离")
	}
	tag = decode[model.Tag](t, a.call("PATCH", "/api/tags/"+tag.ID, owner.Token, `{"name":"毕业设计","color":"#6750A4"}`, 200))
	book := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	path := "/api/notebooks/" + book.ID + "/tags"
	a.call("PUT", path, other.Token, `{"tagIds":[]}`, 404)
	a.call("PUT", path, owner.Token, `{"tagIds":["`+foreign.ID+`"]}`, 404)
	for _, body := range []string{`{}`, `{"tagIds":null}`, `{"tagIds":["bad"]}`} {
		a.call("PUT", path, owner.Token, body, 400)
	}
	book = decode[service.NotebookView](t, a.call("PUT", path, owner.Token, `{"tagIds":["`+tag.ID+`","`+tag.ID+`"]}`, 200))
	if len(book.Tags) != 1 || book.Tags[0].Name != "毕业设计" {
		t.Fatal("整体替换与去重失败")
	}
	a.call("PUT", path, owner.Token, `{"tagIds":["`+foreign.ID+`"]}`, 404)
	items := decode[[]service.NotebookView](t, a.call("GET", "/api/notebooks?tag="+tag.ID, owner.Token, "", 200))
	if len(items) != 1 || len(items[0].Tags) != 1 {
		t.Fatal("标签过滤失败或失败替换未回滚")
	}
	a.call("GET", "/api/notebooks?tag="+foreign.ID, owner.Token, "", 404)
	a.call("GET", "/api/notebooks?tag=bad", owner.Token, "", 400)
	a.call("PUT", path, owner.Token, `{"tagIds":[]}`, 200)
	if string(a.call("GET", "/api/notebooks?tag="+tag.ID, owner.Token, "", 200)) != "[]" {
		t.Fatal("清空失败")
	}
	a.call("PUT", path, owner.Token, `{"tagIds":["`+tag.ID+`"]}`, 200)
	a.call("DELETE", "/api/tags/"+tag.ID, owner.Token, "", 200)
	if len(decode[service.NotebookView](t, a.call("GET", "/api/notebooks/"+book.ID, owner.Token, "", 200)).Tags) != 0 {
		t.Fatal("删除标签未级联关联")
	}
	request := httptest.NewRequest("OPTIONS", path, nil)
	request.Header.Set("Origin", "http://localhost:5190")
	request.Header.Set("Access-Control-Request-Method", "PUT")
	w := httptest.NewRecorder()
	a.router.ServeHTTP(w, request)
	if !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "PUT") {
		t.Fatal("跨域未开放标签替换方法")
	}
}

func TestT10CitationsMigrationAndCascade(t *testing.T) {
	a, _ := t10API(t)
	owner, other := a.register(), a.register()
	book := decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	path := "/api/notebooks/" + book.ID
	own := model.NotebookOwner{UserID: owner.User.ID, NotebookID: book.ID}
	source := model.Source{NotebookOwner: own, Type: "text", Title: "引用资料", Status: "ready", Content: "标签通过关联表实现多对多关系。"}
	if err := a.database.Create(&source).Error; err != nil {
		t.Fatal(err)
	}
	vec, _ := llm.NewFake(1536).Embed(context.Background(), []string{source.Content})
	page := 2
	chunk := model.Chunk{NotebookOwner: own, SourceID: source.ID, Content: source.Content, Page: &page, LocationLabel: "第 2 页", Embedding: model.NewVector(vec[0])}
	if err := a.database.Create(&chunk).Error; err != nil {
		t.Fatal(err)
	}
	events := streamChat(t, a, path, owner.Token, `{"question":"标签是什么关系？","sourceIds":["`+source.ID+`"]}`)
	if events[len(events)-1].Type != "done" {
		t.Fatal("回答保存失败")
	}
	var rows []model.MessageCitation
	if err := a.database.Where("source_id = ?", source.ID).Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].ChunkID == nil || *rows[0].ChunkID != chunk.ID {
		t.Fatalf("引用关联未写入：%v %+v", err, rows)
	}
	var stored model.Message
	a.database.Where("id = ?", rows[0].MessageID).First(&stored)
	if len(stored.Citations) != 0 {
		t.Fatal("新回答仍写入 JSONB")
	}
	messages := decode[[]model.Message](t, a.call("GET", path+"/messages", owner.Token, "", 200))
	if len(messages[1].Citations) != 1 || messages[1].Citations[0].Page == nil || messages[1].Citations[0].SourceTitle != source.Title {
		t.Fatal("历史引用格式改变")
	}
	a.call("GET", path+"/messages", other.Token, "", 404)
	legacy := model.Message{NotebookOwner: own, Role: "assistant", Content: "旧引用 [3]", Citations: []model.Citation{{Index: 3, SourceID: source.ID, SourceTitle: "旧标题", Quote: "旧摘录", Page: &page, LocationLabel: "第 2 页"}, {Index: 7, SourceID: "已删除的来源", SourceTitle: "删除前标题", Quote: "失效快照"}}}
	if err := a.database.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := a.database.Where("version = ?", "t10_message_citations").Delete(&model.SchemaMigration{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(context.Background(), a.database, 1536); err != nil {
		t.Fatal(err)
	}
	var migrated []model.MessageCitation
	a.database.Where("message_id = ?", legacy.ID).Order("index").Find(&migrated)
	if len(migrated) != 2 || migrated[0].SourceID == nil || migrated[0].ChunkID != nil || migrated[1].SourceID != nil || migrated[1].View().SourceID != "已删除的来源" {
		t.Fatalf("迁移快照或空外键错误：%+v", migrated)
	}
	// 证明一次性标记生效：旧列保留，但删除的新行不会在重启后复活。
	a.database.Where("id = ?", migrated[1].ID).Delete(&model.MessageCitation{})
	if err := db.Migrate(context.Background(), a.database, 1536); err != nil {
		t.Fatal(err)
	}
	var count int64
	a.database.Model(&model.MessageCitation{}).Where("message_id = ?", legacy.ID).Count(&count)
	if count != 1 {
		t.Fatal("迁移重复执行")
	}
	if err := a.database.Where("id = ?", source.ID).Delete(&model.Source{}).Error; err != nil {
		t.Fatal(err)
	}
	rows = nil
	a.database.Where("message_id = ?", stored.ID).Find(&rows)
	if len(rows) != 1 || rows[0].SourceID != nil || rows[0].ChunkID != nil || rows[0].SourceTitle != source.Title || rows[0].Quote == "" {
		t.Fatal("SET NULL 丢失快照")
	}
	tag := decode[model.Tag](t, a.call("POST", "/api/tags", owner.Token, `{"name":"级联"}`, 200))
	a.call("PUT", path+"/tags", owner.Token, `{"tagIds":["`+tag.ID+`"]}`, 200)
	a.call("DELETE", path, owner.Token, "", 200)
	a.database.Model(&model.MessageCitation{}).Where("message_id IN ?", []string{stored.ID, legacy.ID}).Count(&count)
	if count != 0 {
		t.Fatal("笔记本删除未级联引用")
	}
	a.database.Model(&model.NotebookTag{}).Where("notebook_id = ?", book.ID).Count(&count)
	if count != 0 {
		t.Fatal("笔记本删除未级联标签关系")
	}
	// 删除用户前再创建整条关联链，避免只断言空表。
	book = decode[service.NotebookView](t, a.call("POST", "/api/notebooks", owner.Token, `{}`, 200))
	seedNotebookData(t, a.database, owner.User.ID, book.ID)
	var message model.Message
	a.database.Where("user_id = ? AND notebook_id = ?", owner.User.ID, book.ID).First(&message)
	link := model.MessageCitation{MessageID: message.ID, Index: 1, SourceRef: "快照", SourceTitle: "标题", Quote: "正文"}
	if err := a.database.Omit(clause.Associations).Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	a.call("PUT", "/api/notebooks/"+book.ID+"/tags", owner.Token, `{"tagIds":["`+tag.ID+`"]}`, 200)
	if err := a.database.Where("id = ?", owner.User.ID).Delete(&model.User{}).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"notebooks", "user_settings", "tags", "sources", "chunks", "messages", "notes", "artifacts", "audio_overviews", "notebook_guides", "login_logs"} {
		a.database.Table(table).Where("user_id = ?", owner.User.ID).Count(&count)
		if count != 0 {
			t.Fatalf("用户删除未级联 %s", table)
		}
	}
	a.database.Model(&model.MessageCitation{}).Where("id = ?", link.ID).Count(&count)
	if count != 0 {
		t.Fatal("用户删除残留引用")
	}
	a.database.Model(&model.NotebookTag{}).Where("notebook_id = ?", book.ID).Count(&count)
	if count != 0 {
		t.Fatal("用户删除残留标签关系")
	}
}

func TestT10AuthLogsAndRateLimit(t *testing.T) {
	a, core := t10API(t)
	owner, other := a.register(), a.register()
	body := `{"email":"` + strings.ToUpper(owner.User.Email) + `","password":"wrong"}`
	a.call("POST", "/api/auth/register", "", `{"email":"`+owner.User.Email+`","password":"password","nickname":"重复"}`, 409)
	a.call("POST", "/api/auth/register", "", `{"email":"`+owner.User.Email+`","password":"abc","nickname":"弱密码"}`, 400)
	for i := 0; i < 3; i++ {
		a.call("POST", "/api/auth/login", "", body, 401)
	}
	a.call("POST", "/api/auth/login", "", body, 429)
	a.call("POST", "/api/auth/login", "", `{"email":"`+owner.User.Email+`","password":"test-password"}`, 429)
	logs := decode[[]model.LoginLog](t, a.call("GET", "/api/me/login-logs", owner.Token, "", 200))
	if len(logs) != 8 || !logs[len(logs)-1].Success {
		t.Fatalf("认证成功/失败没有全部记录：%d", len(logs))
	}
	for _, entry := range logs {
		if entry.Email != owner.User.Email || (!entry.Success && entry.UserID != nil) {
			t.Fatal("日志邮箱归一化或失败归属错误")
		}
	}
	var nullCount int64
	a.database.Model(&model.LoginLog{}).Where("email = ? AND success = false AND user_id IS NULL", owner.User.Email).Count(&nullCount)
	if nullCount != 7 {
		t.Fatal("失败日志必须无 user_id")
	}
	if len(decode[[]model.LoginLog](t, a.call("GET", "/api/me/login-logs", other.Token, "", 200))) != 1 {
		t.Fatal("登录记录泄露其他邮箱")
	}
	a.call("GET", "/api/me/login-logs", "", "", 401)
	if err := a.database.Model(&model.LoginLog{}).Where("email = ?", owner.User.Email).Update("created_at", time.Now().Add(-11*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	a.call("POST", "/api/auth/login", "", `{"email":"`+owner.User.Email+`","password":"test-password"}`, 200)
	// 并发请求也只能留下五次未限流失败；第六次起返回 429。
	var failures, limited atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := core.Authenticate(context.Background(), other.User.Email, "wrong", "", "127.0.0.1", "并发测试", false, nil)
			if err == service.ErrUnauthorized {
				failures.Add(1)
			} else if err == service.ErrAuthLimited {
				limited.Add(1)
			} else {
				t.Errorf("异常结果：%v", err)
			}
		}()
	}
	wg.Wait()
	if failures.Load() != 5 || limited.Load() != 3 {
		t.Fatalf("并发限流失效：%d/%d", failures.Load(), limited.Load())
	}
	for i := 0; i < 25; i++ {
		entry := model.LoginLog{Email: owner.User.Email, Success: false, IP: "127.0.0.1", UserAgent: "测试"}
		if err := a.database.Create(&entry).Error; err != nil {
			t.Fatal(err)
		}
	}
	if len(decode[[]model.LoginLog](t, a.call("GET", "/api/me/login-logs", owner.Token, "", 200))) != 20 {
		t.Fatal("最近20条限制失效")
	}
	raw, _ := json.Marshal(logs)
	if strings.Contains(string(raw), "userId") {
		t.Fatal("登录日志暴露内部归属字段")
	}
}

func TestT10ForeignKeyIndexes(t *testing.T) {
	a, _ := t10API(t)
	var missing []string
	// 外键全部列必须是某个有效 B-tree 索引的前导列，等值查询允许列顺序不同。
	err := a.database.Raw(`SELECT c.conname FROM pg_constraint c JOIN pg_class t ON t.oid=c.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace
 WHERE c.contype='f' AND n.nspname=current_schema() AND NOT EXISTS (
 SELECT 1 FROM pg_index i JOIN pg_class idx ON idx.oid=i.indexrelid JOIN pg_am am ON am.oid=idx.relam
 WHERE i.indrelid=c.conrelid AND i.indisvalid AND i.indpred IS NULL AND am.amname='btree'
 AND ARRAY(SELECT unnest(i.indkey::smallint[]) LIMIT cardinality(c.conkey)) @> c.conkey)`).Scan(&missing).Error
	if err != nil || len(missing) > 0 {
		t.Fatalf("外键缺少完整索引：%v %v", err, missing)
	}
}
