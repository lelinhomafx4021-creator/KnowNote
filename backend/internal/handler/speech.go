package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"knownote/internal/llm"
	"knownote/internal/middleware"
	"knownote/pkg/response"
)

func (h *Handler) SpeechConfig(c *gin.Context) {
	enabled := h.Service.Usage != nil && h.Service.Usage.ASREnabled()
	response.OK(c, gin.H{"enabled": enabled, "maxSeconds": llm.SpeechMaxSeconds})
}

func (h *Handler) Transcribe(c *gin.Context) {
	guard := h.Service.Usage
	if guard == nil || !guard.ASREnabled() {
		response.Fail(c, http.StatusServiceUnavailable, 50301, llm.ErrASRNotConfigured.Error())
		return
	}
	// 流式读取全部字段，只保存在内存中；禁止 ParseMultipartForm 创建临时音频文件。
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, llm.SpeechMaxBytes+(64<<10))
	reader, err := c.Request.MultipartReader()
	if err != nil {
		invalid(c, "请使用表单上传 WAV 录音")
		return
	}
	var audio []byte
	var language string
	var haveAudio, haveLanguage bool
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			speechUploadError(c, err)
			return
		}
		switch part.FormName() {
		case "audio":
			if haveAudio || part.FileName() == "" {
				invalid(c, "请上传一份 WAV 录音文件")
				return
			}
			haveAudio = true
			audio, err = io.ReadAll(io.LimitReader(part, llm.SpeechMaxBytes+1))
			if len(audio) > llm.SpeechMaxBytes {
				invalid(c, "录音文件不能超过 4MB")
				return
			}
		case "language":
			if haveLanguage || part.FileName() != "" {
				invalid(c, "语言参数格式不正确")
				return
			}
			haveLanguage = true
			var value []byte
			value, err = io.ReadAll(io.LimitReader(part, 1025))
			if len(value) > 1024 {
				invalid(c, "语言参数过长")
				return
			}
			language = strings.TrimSpace(string(value))
		default:
			_, err = io.Copy(io.Discard, part)
		}
		if err != nil {
			speechUploadError(c, err)
			return
		}
		if err := part.Close(); err != nil {
			speechUploadError(c, err)
			return
		}
	}
	if !haveAudio || len(audio) == 0 {
		invalid(c, "请上传非空 WAV 录音文件")
		return
	}
	text, err := guard.Transcribe(llm.WithUser(c.Request.Context(), middleware.UserID(c)), audio, language)
	var wavErr *llm.WAVError
	var limit *llm.LimitError
	switch {
	case err == nil:
		response.OK(c, gin.H{"text": text})
	case errors.As(err, &wavErr):
		invalid(c, wavErr.Message)
	case errors.As(err, &limit):
		response.Fail(c, http.StatusTooManyRequests, 42901, limit.Message)
	case errors.Is(err, llm.ErrASRNotConfigured):
		response.Fail(c, http.StatusServiceUnavailable, 50301, llm.ErrASRNotConfigured.Error())
	case errors.Is(err, llm.ErrNoSpeech):
		response.Fail(c, http.StatusUnprocessableEntity, 42201, llm.ErrNoSpeech.Error())
	case errors.Is(err, llm.ErrASRTimeout):
		response.Fail(c, http.StatusGatewayTimeout, response.CodeInternal, llm.ErrASRTimeout.Error())
	case errors.Is(err, llm.ErrASRBusy):
		response.Fail(c, http.StatusBadGateway, response.CodeInternal, llm.ErrASRBusy.Error())
	default:
		response.Fail(c, http.StatusBadGateway, response.CodeInternal, llm.ErrASRUpstream.Error())
	}
}

func speechUploadError(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		invalid(c, "上传内容过大，录音文件不能超过 4MB")
		return
	}
	invalid(c, "录音上传不完整，请重试")
}
