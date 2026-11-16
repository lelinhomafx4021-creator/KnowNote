package llm

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type Fake struct{ dimension int }

var _ Client = (*Fake)(nil)

func NewFake(dimension int) *Fake { return &Fake{dimension: dimension} }
func (*Fake) Name() string        { return "fake" }

var numberedSource = regexp.MustCompile(`\[([1-9][0-9]*)\]`)
var ragSourceHeader = regexp.MustCompile(`(?m)^\[([1-9][0-9]*)\] 来源：`)

func (*Fake) Chat(ctx context.Context, messages []Message, _ ChatOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(messages) >= 2 && strings.HasPrefix(messages[0].Content, StudioPrompt) {
		var sources []StudioSource
		if err := json.Unmarshal([]byte(messages[1].Content), &sources); err != nil || len(sources) == 0 {
			return "", fmt.Errorf("工作室来源格式无效")
		}
		kind := strings.TrimPrefix(strings.SplitN(messages[0].Content, "\n", 2)[0], StudioPrompt)
		focus := ""
		for _, line := range strings.Split(messages[0].Content, "\n") {
			if rest, ok := strings.CutPrefix(line, "本次聚焦需求（请重点围绕该主题结合 RAG 检索原文展开）："); ok {
				focus = strings.TrimSpace(rest)
				break
			}
		}
		return fakeStudioFocused(kind, focus, sources)
	}
	if len(messages) >= 2 && messages[0].Role == "system" && strings.HasPrefix(messages[0].Content, SourceSummaryPrompt+"\n") {
		data, err := json.Marshal(FallbackSourceSummary(messages[len(messages)-1].Content))
		return string(data), err
	}
	if len(messages) >= 2 && strings.HasPrefix(messages[0].Content, NotebookGuidePrompt+"\n") {
		var sources []GuideSource
		if err := json.Unmarshal([]byte(messages[len(messages)-1].Content), &sources); err != nil {
			return "", fmt.Errorf("概要来源格式无效")
		}
		data, err := json.Marshal(FallbackGuide(sources))
		return string(data), err
	}
	question := "请概括当前资料"
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			question = excerpt(messages[i].Content, 100)
			break
		}
	}
	var points []string
	seen := map[string]bool{}
	contextMessages := messages
	sourcePattern := numberedSource
	for _, message := range messages {
		if message.Role == "system" && strings.HasPrefix(message.Content, "KNOWNOTE_RAG_CONTEXT\n") {
			contextMessages = []Message{message}
			sourcePattern = ragSourceHeader
			question = numberedSource.ReplaceAllString(question, "")
			break
		}
	}
	for _, message := range contextMessages {
		matches := sourcePattern.FindAllStringIndex(message.Content, -1)
		for i, position := range matches {
			marker := numberedSource.FindString(message.Content[position[0]:position[1]])
			if seen[marker] {
				continue
			}
			seen[marker] = true
			end := len(message.Content)
			if i+1 < len(matches) {
				end = matches[i+1][0]
			}
			text := excerpt(message.Content[position[1]:end], 150)
			if text == "" {
				text = "可结合该资料中的定义、例子与结论理解问题"
			}
			points = append(points, fmt.Sprintf("- %s %s", text, marker))
		}
	}
	if len(points) == 0 {
		return fmt.Sprintf("关于“%s”，可以先明确核心概念，再整理关键事实和具体例子。\n\n当前没有提供带编号的资料片段；补充资料后，我会结合原文给出带引用的分析。\n\n这是离线演示回答，建议根据资料核对具体结论。", question), nil
	}
	return fmt.Sprintf("围绕“%s”，资料提供了以下线索：\n\n%s\n\n可以将这些要点联系起来，比较它们的适用条件，并回到引用原文核对细节。以上为基于输入片段整理的离线演示回答。", question, strings.Join(points, "\n")), nil
}

func (f *Fake) ChatStream(ctx context.Context, messages []Message, opts ChatOptions, onDelta func(string) error) (string, error) {
	if onDelta == nil {
		return "", fmt.Errorf("流式输出回调不能为空")
	}
	answer, err := f.Chat(ctx, messages, opts)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	for _, character := range answer {
		if err := ctx.Err(); err != nil {
			return output.String(), err
		}
		text := string(character)
		output.WriteString(text)
		if err := onDelta(text); err != nil {
			return output.String(), err
		}
		// 离线模式保留可感知的流式输出，便于用户停止并验证部分回答保存。
		timer := time.NewTimer(3 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return output.String(), ctx.Err()
		case <-timer.C:
		}
	}
	return output.String(), nil
}

func (f *Fake) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if f.dimension < 1 {
		return nil, fmt.Errorf("向量维度必须大于零")
	}
	vectors := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += EmbedBatchSize {
		end := min(start+EmbedBatchSize, len(texts))
		for _, text := range texts[start:end] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			values := make([]float32, f.dimension)
			tokens := tokenize(text)
			if len(tokens) == 0 {
				tokens = []string{"空文本"}
			}
			// 词袋特征散列到相同坐标，共享词和中文相邻字会提高余弦相似度。
			for _, token := range tokens {
				hash := sha256.Sum256([]byte(token))
				for i := 0; i < 4; i++ {
					index := binary.LittleEndian.Uint32(hash[i*8:i*8+4]) % uint32(f.dimension)
					values[index] += 1
				}
			}
			var norm float64
			for _, value := range values {
				norm += float64(value) * float64(value)
			}
			norm = math.Sqrt(norm)
			for i := range values {
				values[i] = float32(float64(values[i]) / norm)
			}
			vectors = append(vectors, values)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return vectors, nil
}

func tokenize(text string) []string {
	var tokens []string
	var word strings.Builder
	var previousHan rune
	flush := func() {
		if word.Len() > 0 {
			tokens = append(tokens, word.String())
			word.Reset()
		}
	}
	for _, r := range strings.ToLower(text) {
		if unicode.Is(unicode.Han, r) {
			flush()
			tokens = append(tokens, string(r))
			if previousHan != 0 {
				tokens = append(tokens, string([]rune{previousHan, r}))
			}
			previousHan = r
			continue
		}
		previousHan = 0
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			word.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

func excerpt(text string, length int) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) > length {
		return string(runes[:length]) + "……"
	}
	return text
}

func (*Fake) TTS(ctx context.Context, _, _ string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrNotConfigured
}
