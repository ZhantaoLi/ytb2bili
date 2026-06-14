package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ZhantaoLi/bilibili-go-sdk/bilibili"
	"github.com/ZhantaoLi/ytb2bili/internal/storage"
)

func main() {
	// 定义命令行参数
	videoPath := flag.String("video", "", "视频文件路径 (必填)")
	coverPath := flag.String("cover", "", "封面图片路径 (可选)")
	title := flag.String("title", "测试上传视频", "视频标题")
	desc := flag.String("desc", "这是一个通过API上传的测试视频", "视频简介")
	loginFile := flag.String("login", "login_info.json", "登录信息文件路径")
	flag.Parse()

	// 检查必要参数
	if *videoPath == "" {
		fmt.Println("请提供视频文件路径")
		fmt.Println("用法示例: go run bin/bili_up_demo.go -video ./test.mp4 -title '测试视频'")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 1. 初始化存储并加载登录信息
	storePath := *loginFile

	// 如果用户没有指定特定路径（即使用了默认值），则尝试智能查找
	if storePath == "login_info.json" {
		if _, err := os.Stat(storePath); os.IsNotExist(err) {
			// 1. 尝试查找系统默认存储位置 ~/.bili_up/login.json
			homeDir, _ := os.UserHomeDir()
			defaultSysPath := filepath.Join(homeDir, ".bili_up", "login.json")
			if _, err := os.Stat(defaultSysPath); err == nil {
				storePath = defaultSysPath
			} else {
				// 2. 尝试在上级目录查找
				altPath := filepath.Join("..", storePath)
				if _, err := os.Stat(altPath); err == nil {
					storePath = altPath
				}
			}
		}
	}

	fmt.Printf("正在加载登录信息: %s\n", storePath)
	store := storage.NewLoginStore(storePath)

	if !store.IsValid() {
		log.Fatalf("❌ 登录信息无效或文件不存在。请先通过Web端扫码登录，确保 %s 存在。\n", storePath)
	}

	loginInfo, err := store.Load()
	if err != nil {
		log.Fatalf("❌ 加载登录信息失败: %v\n", err)
	}

	fmt.Printf("✅ 登录成功! 用户MID: %d, 用户名: %s\n", loginInfo.TokenInfo.Mid, loginInfo.TokenInfo.Uname)

	// 2. 创建上传客户端
	client := bilibili.NewUploadClient(loginInfo)

	// 3. 上传视频
	fmt.Printf("🚀 开始上传视频: %s\n", *videoPath)
	video, err := client.UploadVideo(*videoPath)
	if err != nil {
		log.Fatalf("❌ 上传视频失败: %v\n", err)
	}
	fmt.Printf("✅ 视频上传完成! Filename: %s\n", video.Filename)

	// 4. 上传封面 (如果有)
	coverURL := ""
	if *coverPath != "" {
		fmt.Printf("📸 上传封面: %s\n", *coverPath)
		url, err := client.UploadCover(*coverPath)
		if err != nil {
			log.Printf("⚠️ 封面上传失败 (将使用默认封面): %v\n", err)
		} else {
			coverURL = url
			fmt.Printf("✅ 封面上传成功: %s\n", coverURL)
		}
	}

	// 5. 提交投稿
	fmt.Println("📝 正在提交投稿信息...")

	// 这里使用默认的分区 TID=17 (单机游戏)，实际使用中可能需要配置
	studio := &bilibili.Studio{
		Copyright:    1, // 1=自制
		Title:        *title,
		Desc:         *desc,
		Tag:          "测试上传,Bilibili API",
		Tid:          122, // 分区ID
		Cover:        coverURL,
		Videos:       []bilibili.Video{*video},
		Dynamic:      fmt.Sprintf("发布了新视频：%s", *title),
		NoReprint:    1, // 禁止转载
		OpenSubtitle: false,
	}

	result, err := client.SubmitVideo(studio)
	if err != nil {
		log.Fatalf("❌ 提交投稿失败: %v\n", err)
	}

	if result.Code != 0 {
		log.Fatalf("❌ 提交返回错误: Code=%d, Message=%s\n", result.Code, result.Message)
	}

	// 6. 输出结果
	fmt.Println("🎉 投稿成功!")
	if data, ok := result.Data.(map[string]interface{}); ok {
		if bvid, ok := data["bvid"]; ok {
			fmt.Printf("📺 BVID: %s\n", bvid)
			fmt.Printf("🔗 视频链接: https://www.bilibili.com/video/%s\n", bvid)
		}
		if aid, ok := data["aid"]; ok {
			fmt.Printf("🆔 AID: %v\n", aid)
		}
	} else {
		fmt.Printf("Result Data: %v\n", result.Data)
	}
}
