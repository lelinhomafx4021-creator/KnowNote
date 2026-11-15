package studio

import "testing"

func TestParseSlideDeck(t *testing.T) {
	raw := "\ufeff```json\n{\"title\":\"操作系统课件\",\"subtitle\":\"第3章 进程调度\",\"slides\":[{\"title\":\"先来先服务\",\"bullets\":[\"非抢占式队列调度\",\"长作业易造成护航效应\",],\"notes\":\"举批处理系统为例。\"},{\"title\":\"\",\"bullets\":[]},],}\n```"
	deck, md, err := ParseSlideDeck(raw)
	if err != nil {
		t.Fatalf("ParseSlideDeck 失败: %v", err)
	}
	if deck.Title != "操作系统课件" || len(deck.Slides) != 1 {
		t.Fatalf("PPT 结构解析异常: %+v", deck)
	}
	if len(deck.Slides[0].Bullets) != 2 || deck.Slides[0].Notes == "" {
		t.Fatalf("PPT 条目或备注解析异常: %+v", deck.Slides[0])
	}
	if md == "" {
		t.Fatal("未生成 PPT Markdown 文本快照")
	}
	if _, _, err := ParseSlideDeck(`{"title":"空","slides":[]}`); err == nil {
		t.Fatal("空幻灯片应返回错误")
	}
}
