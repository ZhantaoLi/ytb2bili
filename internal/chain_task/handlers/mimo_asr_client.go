package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asrSegment struct {
	Start    float64
	Duration float64
	Text     string
}

type MimoASRClientConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout int
}

type MimoASRClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

type mimoASRRequest struct {
	Model      string           `json:"model"`
	Messages   []mimoASRMessage `json:"messages"`
	ASROptions mimoASROptions   `json:"asr_options,omitempty"`
	Stream     bool             `json:"stream"`
}

type mimoASRMessage struct {
	Role    string               `json:"role"`
	Content []mimoASRContentPart `json:"content"`
}

type mimoASRContentPart struct {
	Type       string         `json:"type"`
	InputAudio mimoInputAudio `json:"input_audio"`
}

type mimoInputAudio struct {
	Data string `json:"data"`
}

type mimoASROptions struct {
	Language string `json:"language,omitempty"`
}

type mimoASRResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *OpenAIError `json:"error,omitempty"`
}

func NewMimoASRClient(config MimoASRClientConfig) *MimoASRClient {
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = "https://ai.muapi.cn/v1"
	}
	if strings.TrimSpace(config.Model) == "" {
		config.Model = "mimo-v2.5-asr"
	}
	if config.Timeout <= 0 {
		config.Timeout = 120
	}

	return &MimoASRClient{
		apiKey:  config.APIKey,
		baseURL: strings.TrimSuffix(config.BaseURL, "/"),
		model:   config.Model,
		client:  &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
	}
}

func (c *MimoASRClient) TranscribeFile(audioPath, language string) (string, error) {
	audioBytes, err := os.ReadFile(audioPath)
	if err != nil {
		return "", fmt.Errorf("读取音频片段失败: %v", err)
	}

	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(audioPath)))
	if mimeType == "" {
		mimeType = "audio/mpeg"
	}
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(audioBytes))

	requestBody := mimoASRRequest{
		Model: c.model,
		Messages: []mimoASRMessage{
			{
				Role: "user",
				Content: []mimoASRContentPart{
					{
						Type:       "input_audio",
						InputAudio: mimoInputAudio{Data: dataURI},
					},
				},
			},
		},
		ASROptions: mimoASROptions{Language: language},
		Stream:     false,
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化 ASR 请求失败: %v", err)
	}

	apiURL := c.baseURL
	if !strings.Contains(apiURL, "/chat/completions") {
		apiURL = strings.TrimSuffix(apiURL, "/") + "/chat/completions"
	}
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("创建 ASR 请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送 ASR 请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 ASR 响应失败: %v", err)
	}

	var parsed mimoASRResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("解析 ASR 响应失败: %v, 原始响应: %s", err, string(body))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("ASR API 错误 [%s]: %s", parsed.Error.Type, parsed.Error.Message)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ASR API 状态码 %d: %s", resp.StatusCode, string(body))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("ASR 响应中没有转写结果")
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func buildSRTFromASRSegments(segments []asrSegment) string {
	var builder strings.Builder
	index := 1
	for _, segment := range segments {
		text := strings.TrimSpace(segment.Text)
		if text == "" {
			continue
		}

		builder.WriteString(fmt.Sprintf("%d\n", index))
		builder.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTimestamp(segment.Start), formatSRTTimestamp(segment.Start+segment.Duration)))
		builder.WriteString(text)
		builder.WriteString("\n\n")
		index++
	}
	return builder.String()
}

func formatSRTTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMilliseconds := int(seconds*1000 + 0.5)
	hours := totalMilliseconds / 3600000
	totalMilliseconds %= 3600000
	minutes := totalMilliseconds / 60000
	totalMilliseconds %= 60000
	secs := totalMilliseconds / 1000
	milliseconds := totalMilliseconds % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, milliseconds)
}
