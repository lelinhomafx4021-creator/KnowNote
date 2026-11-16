package llm

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// 离线词表采用最长匹配，避免逐字滑窗产生“合大”一类跨词碎片；不依赖分词服务。
var keywordLexicon = strings.Fields(`
知识管理 个人资料 资料管理 资料检索 信息检索 语义检索 向量检索 全文检索 混合检索
检索增强生成 大语言模型 大模型 语言模型 人工智能 机器学习 深度学习 自然语言处理
毕业设计 系统设计 架构设计 数据库 数据隔离 权限隔离 身份认证 用户认证 用户设置
向量数据库 向量索引 向量化 嵌入模型 余弦相似度 相似度 索引 检索 召回 排序
文本切分 文本处理 文本分块 文本解析 文本摘要 重叠 切分 分块 摘要 主题 引用标注 引用校验
流式回答 流式输出 流式传输 流式 对话 问答 笔记本 笔记 来源 网页 文档 论文 参考文献
学习指南 简报 常见问题 时间线 思维导图 语音合成 工作室 转写 字幕
训练数据 验证数据 训练 模型 泛化能力 评估 测试 数据 安全 隐私 缓存 并发 事务
前端 后端 接口 服务 部署 队列 异步 处理 流水线 性能 延迟 存储 文件 上传
经济 教育 医疗 历史 文化 科学 技术 政策 环境 能源 市场 企业 管理 学习 研究
`)

func sourceKeywords(text string) []string {
	frequency := make(map[string]int)
	stop := map[string]bool{"the": true, "and": true, "for": true, "with": true, "this": true, "from": true, "that": true, "are": true, "was": true, "not": true, "com": true, "https": true, "http": true, "is": true, "in": true, "to": true, "of": true, "on": true, "it": true, "as": true, "be": true, "or": true, "an": true}
	flush := func(word []rune) {
		if len(word) < 2 {
			return
		}
		if !unicode.Is(unicode.Han, word[0]) {
			// 数字可属于 GPT4 等名称，但不能单独成为主题。
			key := string(word)
			if len(word) <= 30 && !stop[key] && strings.ContainsFunc(key, unicode.IsLetter) {
				frequency[key]++
			}
			return
		}
		matched := false
		for remaining := string(word); remaining != ""; {
			best := ""
			for _, candidate := range keywordLexicon {
				if len(candidate) > len(best) && strings.HasPrefix(remaining, candidate) {
					best = candidate
				}
			}
			if best == "" {
				_, size := utf8.DecodeRuneInString(remaining)
				remaining = remaining[size:]
				continue
			}
			frequency[best]++
			matched = true
			remaining = remaining[len(best):]
		}
		// 词表外仅保留自然分隔的短词，不从长句中猜测双字组合。
		if !matched && len(word) <= 8 && !strings.ContainsAny(string(word), "的了是在和与及或为将把这那有可也就") {
			frequency[string(word)]++
		}
	}
	var word []rune
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if len(word) > 0 && unicode.Is(unicode.Han, word[0]) != unicode.Is(unicode.Han, r) {
				flush(word)
				word = nil
			}
			word = append(word, r)
		} else {
			flush(word)
			word = nil
		}
	}
	flush(word)
	keys := make([]string, 0, len(frequency))
	for key := range frequency {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if frequency[keys[i]] == frequency[keys[j]] {
			return keys[i] < keys[j]
		}
		return frequency[keys[i]] > frequency[keys[j]]
	})
	topics := append([]string{}, keys[:min(6, len(keys))]...)
	for _, extra := range []string{"资料概览", "核心内容", "原文要点"} {
		if len(topics) >= 3 {
			break
		}
		if frequency[extra] == 0 {
			topics = append(topics, extra)
		}
	}
	return topics
}
