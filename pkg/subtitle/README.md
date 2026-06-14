# YouTube 字幕下载工具 (yt-dlp)

使用 `yt-dlp` 从 YouTube 视频下载字幕文件的 Go 语言封装库。

## 📋 目录

- [快速开始](#快速开始)
- [安装依赖](#安装依赖)
- [API 文档](#api-文档)
- [使用示例](#使用示例)
- [支持的语言](#支持的语言)
- [支持的格式](#支持的格式)
- [常见问题](#常见问题)

## 🚀 快速开始

```go
package main

import (
    "log"
    "github.com/ZhantaoLi/ytb2bili_prod/pkg/subtitle"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()
    downloader := subtitle.NewYtdlpSubtitleDownloader(logger)
    
    // 下载英文字幕
    file, err := downloader.DownloadEnglishSubtitle(
        "https://www.youtube.com/watch?v=VIDEO_ID",
        "srt",              // 格式
        "./output/video",   // 输出路径（不含扩展名）
    )
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("字幕已下载: %s", file)
}
```

## 📦 安装依赖

### 1. 安装 yt-dlp

```bash
# macOS (推荐)
brew install yt-dlp

# Ubuntu/Debian
sudo apt install yt-dlp

# 使用 pip
pip install yt-dlp

# 验证安装
yt-dlp --version
```

### 2. Go 依赖

```bash
go get github.com/sirupsen/logrus
```

## 📖 API 文档

### NewYtdlpSubtitleDownloader

创建字幕下载器实例。

```go
func NewYtdlpSubtitleDownloader(logger *logrus.Logger) *YtdlpSubtitleDownloader
```

**参数:**
- `logger` - logrus 日志实例

**返回:**
- `*YtdlpSubtitleDownloader` - 下载器实例

---

### ListSubtitles

列出视频所有可用字幕。

```go
func (d *YtdlpSubtitleDownloader) ListSubtitles(videoURL string) (*VideoSubtitles, error)
```

**参数:**
- `videoURL` - YouTube 视频 URL

**返回:**
- `*VideoSubtitles` - 包含所有字幕信息的结构体
- `error` - 错误信息

**示例:**
```go
subtitles, err := downloader.ListSubtitles("https://youtube.com/watch?v=VIDEO_ID")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("视频: %s\n", subtitles.Title)
fmt.Printf("时长: %.0f 秒\n", subtitles.Duration)

// 遍历手动字幕
for lang, subs := range subtitles.Subtitles {
    fmt.Printf("语言 %s: %d 个版本\n", lang, len(subs))
}

// 遍历自动字幕
for lang, subs := range subtitles.AutoSubtitles {
    fmt.Printf("自动字幕 %s: %d 个版本\n", lang, len(subs))
}
```

---

### DownloadSubtitle

下载指定语言的字幕。

```go
func (d *YtdlpSubtitleDownloader) DownloadSubtitle(
    videoURL, language, format, outputPath string
) (string, error)
```

**参数:**
- `videoURL` - YouTube 视频 URL
- `language` - 语言代码 (如: `"en"`, `"zh-Hans"`, `"ja"`)
- `format` - 字幕格式 (如: `"srt"`, `"vtt"`, `"json3"`)
- `outputPath` - 输出路径（不含扩展名）

**返回:**
- `string` - 下载的字幕文件路径
- `error` - 错误信息

**示例:**
```go
// 下载日文字幕
file, err := downloader.DownloadSubtitle(
    "https://youtube.com/watch?v=VIDEO_ID",
    "ja",
    "srt",
    "./subtitles/video",
)
```

---

### DownloadEnglishSubtitle

智能下载英文字幕（自动尝试多个语言代码）。

```go
func (d *YtdlpSubtitleDownloader) DownloadEnglishSubtitle(
    videoURL, format, outputPath string
) (string, error)
```

**自动尝试顺序:** `en` → `en-US` → `en-GB`

**示例:**
```go
file, err := downloader.DownloadEnglishSubtitle(
    "https://youtube.com/watch?v=VIDEO_ID",
    "srt",
    "./subtitles/video",
)
```

---

### DownloadChineseSubtitle

智能下载中文字幕（自动尝试多个语言代码）。

```go
func (d *YtdlpSubtitleDownloader) DownloadChineseSubtitle(
    videoURL, format, outputPath string
) (string, error)
```

**自动尝试顺序:** `zh-Hans` → `zh-CN` → `zh-TW` → `zh`

**示例:**
```go
file, err := downloader.DownloadChineseSubtitle(
    "https://youtube.com/watch?v=VIDEO_ID",
    "srt",
    "./subtitles/video",
)
```

---

### DownloadAllSubtitles

下载视频的所有可用字幕。

```go
func (d *YtdlpSubtitleDownloader) DownloadAllSubtitles(
    videoURL, format, outputPath string
) ([]string, error)
```

**参数:**
- `videoURL` - YouTube 视频 URL
- `format` - 字幕格式
- `outputPath` - 输出路径（不含扩展名）

**返回:**
- `[]string` - 所有下载的字幕文件路径列表
- `error` - 错误信息

**示例:**
```go
files, err := downloader.DownloadAllSubtitles(
    "https://youtube.com/watch?v=VIDEO_ID",
    "srt",
    "./subtitles/video",
)

for _, file := range files {
    fmt.Printf("已下载: %s\n", file)
}
```

---

### CheckYtdlpInstalled

检查系统是否已安装 yt-dlp。

```go
func CheckYtdlpInstalled() error
```

**返回:**
- `error` - 如果未安装则返回错误

**示例:**
```go
if err := subtitle.CheckYtdlpInstalled(); err != nil {
    log.Fatal("请先安装 yt-dlp: brew install yt-dlp")
}
fmt.Println("yt-dlp 已安装")
```

## 💡 使用示例

### 示例 1: 基础下载

```go
package main

import (
    "log"
    "github.com/ZhantaoLi/ytb2bili_prod/pkg/subtitle"
    "github.com/sirupsen/logrus"
)

func main() {
    // 检查 yt-dlp
    if err := subtitle.CheckYtdlpInstalled(); err != nil {
        log.Fatal(err)
    }
    
    // 创建下载器
    logger := logrus.New()
    downloader := subtitle.NewYtdlpSubtitleDownloader(logger)
    
    // 下载英文字幕
    file, err := downloader.DownloadEnglishSubtitle(
        "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
        "srt",
        "./downloads/rickroll",
    )
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("✓ 字幕已保存: %s", file)
}
```

### 示例 2: 查看可用字幕

```go
func listAvailableSubtitles(videoURL string) {
    logger := logrus.New()
    downloader := subtitle.NewYtdlpSubtitleDownloader(logger)
    
    // 获取字幕列表
    subtitles, err := downloader.ListSubtitles(videoURL)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("视频: %s\n", subtitles.Title)
    fmt.Printf("时长: %.0f 秒\n\n", subtitles.Duration)
    
    // 显示手动字幕
    if len(subtitles.Subtitles) > 0 {
        fmt.Println("可用手动字幕:")
        for lang := range subtitles.Subtitles {
            fmt.Printf("  - %s\n", lang)
        }
    }
    
    // 显示自动字幕
    if len(subtitles.AutoSubtitles) > 0 {
        fmt.Println("\n可用自动字幕:")
        for lang := range subtitles.AutoSubtitles {
            fmt.Printf("  - %s\n", lang)
        }
    }
}
```

### 示例 3: 下载多语言字幕

```go
func downloadMultipleLanguages(videoURL string) {
    logger := logrus.New()
    downloader := subtitle.NewYtdlpSubtitleDownloader(logger)
    
    languages := []string{"en", "zh-Hans", "ja", "ko"}
    
    for _, lang := range languages {
        file, err := downloader.DownloadSubtitle(
            videoURL,
            lang,
            "srt",
            fmt.Sprintf("./subtitles/%s", lang),
        )
        
        if err != nil {
            log.Printf("⚠️ %s 字幕下载失败: %v", lang, err)
            continue
        }
        
        log.Printf("✓ %s 字幕已下载: %s", lang, file)
    }
}
```

### 示例 4: 带重试的下载

```go
func downloadWithRetry(videoURL, language string, maxRetries int) (string, error) {
    logger := logrus.New()
    downloader := subtitle.NewYtdlpSubtitleDownloader(logger)
    
    for i := 0; i < maxRetries; i++ {
        file, err := downloader.DownloadSubtitle(
            videoURL,
            language,
            "srt",
            "./subtitles/video",
        )
        
        if err == nil {
            return file, nil
        }
        
        log.Printf("尝试 %d/%d 失败: %v", i+1, maxRetries, err)
        
        if i < maxRetries-1 {
            time.Sleep(time.Second * time.Duration(i+1))
        }
    }
    
    return "", fmt.Errorf("下载失败，已重试 %d 次", maxRetries)
}
```

### 示例 5: 优先级下载

```go
func downloadPreferredSubtitle(videoURL string) (string, error) {
    logger := logrus.New()
    downloader := subtitle.NewYtdlpSubtitleDownloader(logger)
    
    // 优先级: 英文 > 中文 > 日文
    preferences := []struct {
        lang string
        name string
    }{
        {"en", "英文"},
        {"zh-Hans", "中文"},
        {"ja", "日文"},
    }
    
    for _, pref := range preferences {
        file, err := downloader.DownloadSubtitle(
            videoURL,
            pref.lang,
            "srt",
            "./subtitles/video",
        )
        
        if err == nil {
            log.Printf("✓ 使用 %s 字幕", pref.name)
            return file, nil
        }
        
        log.Printf("⚠️ 未找到 %s 字幕，尝试下一个", pref.name)
    }
    
    return "", fmt.Errorf("未找到任何可用字幕")
}
```

## 🌐 支持的语言

### 常用语言代码

| 语言 | 主代码 | 备选代码 |
|------|--------|----------|
| 英文 | `en` | `en-US`, `en-GB`, `en-CA` |
| 中文（简体）| `zh-Hans` | `zh-CN`, `zh` |
| 中文（繁体）| `zh-Hant` | `zh-TW`, `zh-HK` |
| 日文 | `ja` | `jp` |
| 韩文 | `ko` | `kr` |
| 西班牙语 | `es` | `es-ES`, `es-MX` |
| 法语 | `fr` | `fr-FR`, `fr-CA` |
| 德语 | `de` | `de-DE` |
| 俄语 | `ru` | - |
| 葡萄牙语 | `pt` | `pt-BR`, `pt-PT` |
| 意大利语 | `it` | `it-IT` |
| 阿拉伯语 | `ar` | - |
| 印地语 | `hi` | - |
| 泰语 | `th` | - |
| 越南语 | `vi` | - |

### 语言代码查找

```go
// 使用 ListSubtitles 查看视频的所有可用语言
subtitles, _ := downloader.ListSubtitles(videoURL)

fmt.Println("可用语言:")
for lang := range subtitles.Subtitles {
    fmt.Printf("  - %s\n", lang)
}
for lang := range subtitles.AutoSubtitles {
    fmt.Printf("  - %s (自动)\n", lang)
}
```

## 📄 支持的格式

### 推荐格式

| 格式 | 扩展名 | 说明 | 兼容性 | 推荐度 |
|------|--------|------|--------|--------|
| **SRT** | `.srt` | SubRip 字幕 | ⭐⭐⭐⭐⭐ | ✅ 推荐 |
| **VTT** | `.vtt` | WebVTT 字幕 | ⭐⭐⭐⭐ | ✅ 推荐 |
| **JSON3** | `.json3` | YouTube 原始格式 | ⭐⭐⭐ | 开发用 |

### 其他格式

| 格式 | 扩展名 | 说明 |
|------|--------|------|
| ASS/SSA | `.ass` | 高级字幕，支持样式 |
| LRC | `.lrc` | 歌词格式 |
| SBV | `.sbv` | YouTube 简单格式 |

### 格式特点对比

```go
// SRT - 最常用，兼容性最好
file, _ := downloader.DownloadSubtitle(videoURL, "en", "srt", "./output")

// VTT - Web 标准，支持样式
file, _ := downloader.DownloadSubtitle(videoURL, "en", "vtt", "./output")

// JSON3 - 包含完整元数据
file, _ := downloader.DownloadSubtitle(videoURL, "en", "json3", "./output")
```

## ❓ 常见问题

### Q1: yt-dlp 未安装怎么办？

**A:** 使用以下命令安装：

```bash
# macOS
brew install yt-dlp

# Ubuntu/Debian
sudo apt install yt-dlp

# pip
pip install yt-dlp
```

### Q2: 视频没有字幕怎么办？

**A:** 先使用 `ListSubtitles()` 检查可用字幕：

```go
subtitles, err := downloader.ListSubtitles(videoURL)
if err != nil {
    // 处理错误
}

if len(subtitles.Subtitles) == 0 && len(subtitles.AutoSubtitles) == 0 {
    log.Println("该视频没有任何字幕")
}
```

### Q3: 下载的文件名格式是什么？

**A:** 文件名格式为：`{outputPath}.{language}.{format}`

例如：
- 输入: `outputPath = "./video/test"`
- 输出: `./video/test.en.srt`

### Q4: 如何下载自动生成的字幕？

**A:** yt-dlp 会自动优先下载手动字幕，如果没有才会下载自动字幕。无需特殊设置。

### Q5: 下载速度慢怎么办？

**A:** 可能是网络问题，可以设置代理：

```bash
export HTTP_PROXY=http://proxy:port
export HTTPS_PROXY=http://proxy:port
```

### Q6: 如何处理下载失败？

**A:** 使用带重试机制的封装：

```go
func downloadWithRetry(downloader *subtitle.YtdlpSubtitleDownloader, 
                       videoURL, lang string) (string, error) {
    maxRetries := 3
    for i := 0; i < maxRetries; i++ {
        file, err := downloader.DownloadSubtitle(videoURL, lang, "srt", "./output")
        if err == nil {
            return file, nil
        }
        log.Printf("重试 %d/%d", i+1, maxRetries)
        time.Sleep(time.Second * 2)
    }
    return "", fmt.Errorf("下载失败")
}
```

### Q7: 能否批量下载多个视频的字幕？

**A:** 可以，使用循环处理：

```go
videoURLs := []string{
    "https://youtube.com/watch?v=VIDEO1",
    "https://youtube.com/watch?v=VIDEO2",
}

for i, url := range videoURLs {
    file, err := downloader.DownloadEnglishSubtitle(url, "srt", fmt.Sprintf("./video_%d", i))
    if err != nil {
        log.Printf("视频 %d 下载失败: %v", i+1, err)
        continue
    }
    log.Printf("✓ 视频 %d 字幕已下载", i+1)
}
```

### Q8: 如何获取字幕的详细信息？

**A:** 使用 `ListSubtitles()` 获取详细信息：

```go
subtitles, _ := downloader.ListSubtitles(videoURL)

for lang, subs := range subtitles.Subtitles {
    for _, sub := range subs {
        fmt.Printf("语言: %s\n", sub.Language)
        fmt.Printf("名称: %s\n", sub.LanguageName)
        fmt.Printf("格式: %s\n", sub.Ext)
        fmt.Printf("URL: %s\n", sub.URL)
        fmt.Printf("自动: %v\n", sub.IsAutomatic)
    }
}
```

## 🔧 故障排查

### 调试模式

启用详细日志：

```go
logger := logrus.New()
logger.SetLevel(logrus.DebugLevel)
logger.SetFormatter(&logrus.TextFormatter{
    FullTimestamp: true,
})

downloader := subtitle.NewYtdlpSubtitleDownloader(logger)
```

### 手动测试 yt-dlp

```bash
# 列出所有字幕
yt-dlp --list-subs "VIDEO_URL"

# 下载英文字幕
yt-dlp --skip-download --write-subs --sub-langs en "VIDEO_URL"

# 查看详细日志
yt-dlp --verbose --skip-download --write-subs "VIDEO_URL"
```

## 📚 相关文档

- [yt-dlp 官方文档](https://github.com/yt-dlp/yt-dlp)
- [YouTube Data API](https://developers.google.com/youtube/v3)
- [SRT 格式规范](https://en.wikipedia.org/wiki/SubRip)

## 📝 许可证

MIT License

---

**最后更新:** 2025年12月3日
