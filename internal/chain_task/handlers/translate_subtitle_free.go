package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultBingAuthEndpoint      = "https://edge.microsoft.com/translate/auth"
	defaultBingTranslateEndpoint = "https://api-edge.cognitive.microsofttranslator.com/translate"
	defaultGoogleTranslateURL    = "https://translate.google.com/m"
)

var (
	freeTranslateHTTPClient   = &http.Client{Timeout: 20 * time.Second}
	freeBingAuthEndpoint      = defaultBingAuthEndpoint
	freeBingTranslateEndpoint = defaultBingTranslateEndpoint
	freeGoogleTranslateURL    = defaultGoogleTranslateURL
	googleResultPattern       = regexp.MustCompile(`(?s)class="(?:t0|result-container)">(.*?)<`)
)

func (t *TranslateSubtitle) translateGroupWithFreeProviders(texts []string) ([]string, error) {
	if translated, err := t.translateGroupWithBing(texts); err == nil {
		return translated, nil
	}
	return t.translateGroupWithGoogle(texts)
}

func (t *TranslateSubtitle) translateGroupWithBing(texts []string) ([]string, error) {
	token, err := t.fetchBingAuthToken()
	if err != nil {
		return nil, err
	}

	bodyItems := make([]map[string]string, 0, len(texts))
	for _, text := range texts {
		bodyItems = append(bodyItems, map[string]string{"Text": truncateForFreeTranslator(text)})
	}
	body, err := json.Marshal(bodyItems)
	if err != nil {
		return nil, err
	}

	endpoint, err := url.Parse(freeBingTranslateEndpoint)
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("to", t.freeTargetLanguage("bing"))
	query.Set("api-version", "3.0")
	query.Set("includeSentenceLength", "true")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", freeTranslateUserAgent())

	resp, err := freeTranslateHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("bing translate status %d: %s", resp.StatusCode, string(respBody))
	}

	var payload []struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil, err
	}
	if len(payload) != len(texts) {
		return nil, fmt.Errorf("bing translate count mismatch: got %d want %d", len(payload), len(texts))
	}

	translated := make([]string, 0, len(payload))
	for i, item := range payload {
		if len(item.Translations) == 0 || strings.TrimSpace(item.Translations[0].Text) == "" {
			return nil, fmt.Errorf("bing translate empty result at %d", i)
		}
		translated = append(translated, strings.TrimSpace(item.Translations[0].Text))
	}
	return translated, nil
}

func (t *TranslateSubtitle) fetchBingAuthToken() (string, error) {
	req, err := http.NewRequest(http.MethodGet, freeBingAuthEndpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", freeTranslateUserAgent())

	resp, err := freeTranslateHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("bing auth status %d: %s", resp.StatusCode, string(body))
	}
	token := strings.TrimSpace(string(body))
	if token == "" {
		return "", fmt.Errorf("bing auth token is empty")
	}
	return token, nil
}

func (t *TranslateSubtitle) translateGroupWithGoogle(texts []string) ([]string, error) {
	translated := make([]string, 0, len(texts))
	for i, text := range texts {
		result, err := t.translateOneWithGoogle(text)
		if err != nil {
			return nil, fmt.Errorf("google translate item %d failed: %w", i+1, err)
		}
		translated = append(translated, result)
	}
	return translated, nil
}

func (t *TranslateSubtitle) translateOneWithGoogle(text string) (string, error) {
	endpoint, err := url.Parse(freeGoogleTranslateURL)
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("tl", t.freeTargetLanguage("google"))
	query.Set("sl", "auto")
	query.Set("q", truncateForFreeTranslator(text))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/4.0 (compatible; MSIE 6.0; Windows NT 5.1)")

	resp, err := freeTranslateHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("google translate status %d: %s", resp.StatusCode, string(body))
	}

	matches := googleResultPattern.FindSubmatch(body)
	if len(matches) < 2 {
		return "", fmt.Errorf("google translate result not found")
	}
	result := strings.TrimSpace(html.UnescapeString(string(matches[1])))
	if result == "" {
		return "", fmt.Errorf("google translate result is empty")
	}
	return result, nil
}

func (t *TranslateSubtitle) freeTargetLanguage(provider string) string {
	target := "ZH"
	if t.App != nil && t.App.Config != nil && t.App.Config.DeepLXConfig != nil && strings.TrimSpace(t.App.Config.DeepLXConfig.TargetLang) != "" {
		target = strings.ToUpper(strings.TrimSpace(t.App.Config.DeepLXConfig.TargetLang))
	}

	switch provider {
	case "google":
		switch target {
		case "ZH", "ZH-CN", "ZH-HANS":
			return "zh-CN"
		case "ZH-TW", "ZH-HANT":
			return "zh-TW"
		default:
			return strings.ToLower(target)
		}
	default:
		switch target {
		case "ZH", "ZH-CN", "ZH-HANS":
			return "zh-Hans"
		case "ZH-TW", "ZH-HANT":
			return "zh-Hant"
		default:
			return strings.ToLower(target)
		}
	}
}

func truncateForFreeTranslator(text string) string {
	runes := []rune(text)
	if len(runes) <= 5000 {
		return text
	}
	return string(runes[:5000])
}

func freeTranslateUserAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0"
}
