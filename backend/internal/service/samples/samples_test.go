package samples_test

import (
	"testing"

	"knownote/internal/service/samples"
	contents "knownote/samples"
)

func TestLoadCuratedManifests(t *testing.T) {
	manifests, err := samples.Load(contents.Content)
	if err != nil {
		t.Fatalf("加载官方示例模板失败: %v", err)
	}
	if len(manifests) < 4 {
		t.Fatalf("预期至少 4 个精选示例笔记本，实际仅 %d 个", len(manifests))
	}
	if manifests[0].Slug != samples.StarterSlug {
		t.Fatalf("初始默认模板顺序错误，预期 %s，实际 %s", samples.StarterSlug, manifests[0].Slug)
	}
	for _, m := range manifests {
		if m.Title == "" || m.Description == "" || len(m.Sources) < 2 {
			t.Fatalf("示例模板字段不完整: %+v", m)
		}
	}
}
