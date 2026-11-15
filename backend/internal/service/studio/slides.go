package studio

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"knownote/internal/exporter"
	"knownote/internal/model"
)

func sanitizeSlideText(text string, limit int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\x00", ""))
	if limit > 0 {
		runes := []rune(text)
		if len(runes) > limit {
			return strings.TrimSpace(string(runes[:limit])) + "…"
		}
	}
	return text
}

// ParseSlideDeck 解析并规整模型生成的 PPT 课件 JSON，并生成对应的 Markdown 文本快照。
func ParseSlideDeck(raw string) (*model.SlideDeck, string, error) {
	cleaned, err := cleanJSONObject(raw, "PPT 课件")
	if err != nil {
		return nil, "", err
	}
	var deck model.SlideDeck
	decoder := json.NewDecoder(strings.NewReader(cleaned))
	if err := decoder.Decode(&deck); err != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, "", fmt.Errorf("PPT 课件 JSON 格式无效")
	}
	deck.Title = sanitizeSlideText(deck.Title, 80)
	deck.Subtitle = sanitizeSlideText(deck.Subtitle, 120)
	if deck.Title == "" {
		deck.Title = "课程专题课件"
	}

	validSlides := make([]model.Slide, 0, min(len(deck.Slides), 20))
	for _, s := range deck.Slides {
		if len(validSlides) >= 20 {
			break
		}
		title := sanitizeSlideText(s.Title, 60)
		subtitle := sanitizeSlideText(s.Subtitle, 80)
		notes := sanitizeSlideText(s.Notes, 600)
		bullets := make([]string, 0, len(s.Bullets))
		for _, b := range s.Bullets {
			clean := sanitizeSlideText(b, 140)
			if clean != "" {
				bullets = append(bullets, clean)
			}
			if len(bullets) >= 8 {
				break
			}
		}
		if title == "" && len(bullets) == 0 {
			continue
		}
		if title == "" {
			title = fmt.Sprintf("核心内容 %d", len(validSlides)+1)
		}
		if len(bullets) == 0 {
			bullets = []string{subtitle}
		}
		validSlides = append(validSlides, model.Slide{
			Title:    title,
			Subtitle: subtitle,
			Bullets:  bullets,
			Notes:    notes,
		})
	}
	if len(validSlides) == 0 {
		return nil, "", fmt.Errorf("PPT 课件缺少有效页面")
	}
	deck.Slides = validSlides
	return &deck, exporter.SlideDeckToMarkdown(&deck), nil
}
