package studio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"knownote/internal/llm"
	"knownote/internal/model"
	"knownote/internal/service"
	"knownote/internal/storage"
)

type AudioView struct {
	NotebookID  string `json:"notebookId"`
	Status      string `json:"status"`
	Title       string `json:"title,omitempty"`
	DurationSec int    `json:"durationSec,omitempty"`
	URL         string `json:"url,omitempty"`
	Transcript  string `json:"transcript,omitempty"`
}

func audioView(item model.AudioOverview) AudioView {
	return AudioView{NotebookID: item.NotebookID, Status: item.Status, Title: item.Title, DurationSec: item.DurationSec, URL: item.URL, Transcript: item.Transcript}
}

func (s *Service) Audio(ctx context.Context, userID, notebookID string) (AudioView, error) {
	if _, err := notebook(s.DB.WithContext(ctx), userID, notebookID, false); err != nil {
		return AudioView{}, err
	}
	var item model.AudioOverview
	err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ?", userID, notebookID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AudioView{NotebookID: notebookID, Status: "idle"}, nil
	}
	return audioView(item), err
}

type StartAudioRequest struct {
	Topic     string
	Preset    string
	SourceIDs []string
}

func (s *Service) StartAudio(ctx context.Context, userID, notebookID string) (AudioView, error) {
	return s.StartAudioWithRequest(ctx, userID, notebookID, StartAudioRequest{})
}

func (s *Service) StartAudioWithRequest(ctx context.Context, userID, notebookID string, req StartAudioRequest) (AudioView, error) {
	topic := strings.TrimSpace(req.Topic)
	if strings.ContainsRune(topic, '\x00') || utf8.RuneCountInString(topic) > 500 {
		return AudioView{}, invalid("对谈主题或要求不能超过500个字符或包含空字符")
	}
	preset := strings.TrimSpace(req.Preset)
	if llm.AudioPresets[preset] == "" {
		preset = ""
	}
	var item model.AudioOverview
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		book, sources, err := s.selectReadySourcesInTx(tx, userID, notebookID, req.SourceIDs, true)
		if err != nil {
			return err
		}
		pickedIDs := []string{}
		if req.SourceIDs != nil {
			pickedIDs = make([]string, len(sources))
			for i, src := range sources {
				pickedIDs[i] = src.ID
			}
		}
		err = tx.Where("user_id = ? AND notebook_id = ?", userID, notebookID).First(&item).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && item.Status == "generating" {
			return nil
		}
		item.NotebookOwner = model.NotebookOwner{UserID: userID, NotebookID: notebookID}
		item.Status, item.Title, item.Preset, item.Prompt, item.SourceIDs, item.Transcript, item.URL, item.DurationSec, item.Error = "generating", "深度对谈："+book.Title, preset, topic, pickedIDs, "", "", 0, ""
		if item.ID == "" {
			return tx.Create(&item).Error
		}
		return tx.Model(&model.AudioOverview{}).Where("user_id = ? AND notebook_id = ? AND id = ?", userID, notebookID, item.ID).
			Select("status", "title", "preset", "prompt", "source_ids", "transcript", "url", "duration_sec", "error").
			Updates(&item).Error
	})
	if err == nil {
		s.notify()
	}
	return audioView(item), err
}

// 单个有界 worker 扫描持久化的 generating 记录；关机取消后重启可继续生成。
func (s *Service) audioWorker(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-ticker.C:
		}
		for ctx.Err() == nil {
			var item model.AudioOverview
			// 与来源队列一致，调度扫描跨用户；实际任务读写均以记录归属约束。
			err := s.DB.WithContext(ctx).Select("id", "user_id", "notebook_id").Where("status = ?", "generating").Order("updated_at,id").First(&item).Error
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) && ctx.Err() == nil {
					s.Logger.Warn("读取音频任务失败，将自动重试")
				}
				break
			}
			s.runAudio(ctx, item)
		}
	}
}

func (s *Service) runAudio(ctx context.Context, key model.AudioOverview) {
	jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	err := s.generateAudio(jobCtx, key)
	if err == nil || ctx.Err() != nil || errors.Is(err, service.ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	// 错误不包含上游响应，避免泄露凭据或资料。
	result := s.DB.WithContext(ctx).Model(&model.AudioOverview{}).Where("user_id = ? AND notebook_id = ? AND id = ? AND status = ?", key.UserID, key.NotebookID, key.ID, "generating").Updates(map[string]any{"status": "error", "error": "音频概览生成失败，请重试"})
	if result.Error != nil {
		s.Logger.Error("保存音频失败状态失败")
	}
}

type audioTurn struct {
	Speaker string
	Text    string
}

func parseTranscript(raw string) ([]audioTurn, error) {
	var turns []audioTurn
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		found := false
		for _, speaker := range []string{"小知", "小页"} {
			for _, colon := range []string{"：", ":"} {
				if rest, ok := strings.CutPrefix(line, speaker+colon); ok {
					rest = strings.TrimSpace(rest)
					if rest == "" {
						found = true // 只有主持人名字没有内容的行直接跳过
						break
					}
					turns = append(turns, audioTurn{speaker, rest})
					seen[speaker] = true
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			// 没有主持人前缀的行：开头归给小知，其余接到上一位主持人的话后面。
			if len(turns) == 0 {
				turns = append(turns, audioTurn{"小知", line})
				seen["小知"] = true
				continue
			}
			turns[len(turns)-1].Text += "\n" + line
		}
	}
	// 不限制篇幅，也不强制两位主持人都发言，只要有内容就采用。
	if len(turns) == 0 {
		return nil, fmt.Errorf("对谈脚本为空")
	}
	return turns, nil
}

func (s *Service) generateAudio(ctx context.Context, key model.AudioOverview) error {
	ctx = llm.WithUser(ctx, key.UserID)
	var current model.AudioOverview
	if err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ? AND id = ?", key.UserID, key.NotebookID, key.ID).First(&current).Error; err != nil {
		return err
	}
	var picked []string
	if len(current.SourceIDs) > 0 {
		picked = current.SourceIDs
	}
	book, sources, err := s.selectReadySources(ctx, key.UserID, key.NotebookID, picked)
	if err != nil {
		return err
	}
	var input string
	if strings.TrimSpace(current.Prompt) != "" {
		input, err = s.ragContext(ctx, key.UserID, key.NotebookID, sources, current.Prompt)
	} else {
		input, err = s.summaryContext(ctx, key.UserID, key.NotebookID, sources, 3)
	}
	if err != nil {
		return err
	}
	opts, err := s.options(ctx, key.UserID)
	if err != nil {
		return err
	}
	messages := llm.AudioMessagesWithFocus(book.Title, current.Prompt, current.Preset, input)
	var turns []audioTurn
	var transcript string
	for attempt := 0; attempt < 2; attempt++ {
		transcript, err = s.Client.Chat(ctx, messages, opts)
		if err != nil {
			return err
		}
		turns, err = parseTranscript(transcript)
		if err == nil {
			break
		}
		messages[0].Content += "\n请严格控制对谈正文总长度为1200~1800字，保留小知、小页两位主持人的标签。"
	}
	if err != nil {
		return err
	}
	// 合成失败时仍保留已经生成的文稿，便于阅读和重试。
	result := s.DB.WithContext(ctx).Model(&model.AudioOverview{}).Where("user_id = ? AND notebook_id = ? AND id = ? AND status = ?", key.UserID, key.NotebookID, key.ID, "generating").Update("transcript", strings.TrimSpace(transcript))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return service.ErrNotFound
	}
	count := 0
	for _, turn := range turns {
		for _, r := range turn.Text {
			if !unicode.IsSpace(r) {
				count++
			}
		}
	}
	duration := max(1, (count+3)/4)
	var audio bytes.Buffer
	if s.TTSModel != "" && llm.TTSEnabled(s.Client) {
		if len(s.Voices) < 2 {
			return fmt.Errorf("请配置两个主持人音色")
		}
		for _, turn := range turns {
			voice := s.Voices[0]
			if turn.Speaker == "小页" {
				voice = s.Voices[1]
			}
			data, err := s.Client.TTS(ctx, turn.Text, voice)
			if err != nil {
				return err
			}
			if len(data) == 0 || int64(audio.Len()+len(data)) > storage.MaxAudioSize {
				return fmt.Errorf("音频片段为空或过大")
			}
			audio.Write(data)
		}
	}
	path := ""
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := notebook(tx, key.UserID, key.NotebookID, true); err != nil {
			return err
		}
		var current model.AudioOverview
		if err := tx.Where("user_id = ? AND notebook_id = ? AND id = ? AND status = ?", key.UserID, key.NotebookID, key.ID, "generating").First(&current).Error; err != nil {
			return err
		}
		url := ""
		if audio.Len() > 0 {
			var err error
			path, err = s.Files.SaveAudio(ctx, key.UserID, key.NotebookID, &audio)
			if err != nil {
				return err
			}
			url = "/api/files/audio/" + key.NotebookID + ".mp3"
		}
		return tx.Model(&model.AudioOverview{}).Where("user_id = ? AND notebook_id = ? AND id = ?", key.UserID, key.NotebookID, key.ID).Updates(map[string]any{"status": "ready", "duration_sec": duration, "url": url, "file_path": path, "error": ""}).Error
	})
	if err != nil && path != "" {
		_ = s.Files.DeleteNotebookFiles(context.Background(), key.UserID, key.NotebookID, []string{path})
	}
	return err
}

func (s *Service) AudioFile(ctx context.Context, userID, filename string) (string, error) {
	id, ok := strings.CutSuffix(filename, ".mp3")
	parsed, parseErr := uuid.Parse(id)
	if !ok || parseErr != nil || parsed == uuid.Nil || parsed.String() != id {
		return "", service.ErrNotFound
	}
	var item model.AudioOverview
	err := s.DB.WithContext(ctx).Where("user_id = ? AND notebook_id = ? AND status = ? AND url <> ''", userID, id, "ready").First(&item).Error
	if err != nil {
		return "", notFound(err)
	}
	if item.FilePath != s.Files.Key(storage.AudioPath(userID, id)) {
		return "", service.ErrNotFound
	}
	return item.FilePath, nil
}
