package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ZhantaoLi/ytb2bili/pkg/store/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BilibiliAccountService B站账号服务
type BilibiliAccountService struct {
	DB            *gorm.DB
	encryptionKey []byte // 用于加密敏感数据
	logger        *zap.SugaredLogger
}

const (
	accountEncryptionKeyEnv      = "YTB2BILI_ACCOUNT_ENCRYPTION_KEY"
	accountEncryptionKeyFilePath = "data/secrets/account_encryption.key"
)

var (
	processAccountEncryptionKey     []byte
	processAccountEncryptionKeyOnce sync.Once
)

// NewBilibiliAccountService 创建B站账号服务实例
func NewBilibiliAccountService(db *gorm.DB, logger *zap.SugaredLogger) *BilibiliAccountService {
	// 生成或加载加密密钥（实际应从配置或环境变量中获取）
	// 必须是16、24或32字节（对应AES-128、AES-192、AES-256）
	key := resolveAccountEncryptionKey(logger)

	return &BilibiliAccountService{
		DB:            db,
		encryptionKey: key,
		logger:        logger,
	}
}

// encrypt 加密敏感数据
func resolveAccountEncryptionKey(logger *zap.SugaredLogger) []byte {
	if key, ok := parseAESKey(os.Getenv(accountEncryptionKeyEnv)); ok {
		return key
	}

	if strings.TrimSpace(os.Getenv(accountEncryptionKeyEnv)) != "" && logger != nil {
		logger.Warnf("%s must be 16, 24, or 32 bytes, or base64 encoding of that length; falling back to the local key file", accountEncryptionKeyEnv)
	}

	processAccountEncryptionKeyOnce.Do(func() {
		key, err := loadOrCreateAccountEncryptionKey(accountEncryptionKeyFilePath)
		if err != nil {
			panic(fmt.Sprintf("load account encryption key: %v", err))
		}
		processAccountEncryptionKey = key
		if logger != nil {
			logger.Infof("%s is not configured; using persistent local key file %s", accountEncryptionKeyEnv, accountEncryptionKeyFilePath)
		}
	})

	return processAccountEncryptionKey
}

func loadOrCreateAccountEncryptionKey(path string) ([]byte, error) {
	content, err := os.ReadFile(path)
	if err == nil {
		if key, ok := parseAESKey(string(content)); ok {
			return key, nil
		}
		return nil, fmt.Errorf("invalid account encryption key file %s", path)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate account encryption key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(key) + "\n"
	if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
		return nil, err
	}
	return key, nil
}

func parseAESKey(raw string) ([]byte, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	if isValidAESKeyLength(len(raw)) {
		return []byte(raw), true
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil && isValidAESKeyLength(len(decoded)) {
		return decoded, true
	}

	return nil, false
}

func isValidAESKeyLength(length int) bool {
	return length == 16 || length == 24 || length == 32
}

func (s *BilibiliAccountService) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt 解密敏感数据
func (s *BilibiliAccountService) decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], string(data[nonceSize:])
	plaintext, err := aesGCM.Open(nil, nonce, []byte(ciphertext), nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// SaveBinding 保存账号绑定
func (s *BilibiliAccountService) SaveBinding(binding *model.AccountBinding) error {
	// 加密敏感数据
	if binding.Cookies != "" {
		encrypted, err := s.encrypt(binding.Cookies)
		if err != nil {
			s.logger.Errorf("加密cookies失败: %v", err)
			return fmt.Errorf("加密cookies失败: %w", err)
		}
		binding.Cookies = encrypted
	}

	if binding.AccessToken != "" {
		encrypted, err := s.encrypt(binding.AccessToken)
		if err != nil {
			s.logger.Errorf("加密access_token失败: %v", err)
			return fmt.Errorf("加密access_token失败: %w", err)
		}
		binding.AccessToken = encrypted
	}

	if binding.RefreshToken != "" {
		encrypted, err := s.encrypt(binding.RefreshToken)
		if err != nil {
			s.logger.Errorf("加密refresh_token失败: %v", err)
			return fmt.Errorf("加密refresh_token失败: %w", err)
		}
		binding.RefreshToken = encrypted
	}

	return s.DB.Create(binding).Error
}

// UpdateBinding 更新账号绑定
func (s *BilibiliAccountService) UpdateBinding(binding *model.AccountBinding, updates map[string]interface{}) error {
	// 加密敏感字段
	if cookies, ok := updates["cookies"].(string); ok && cookies != "" {
		encrypted, err := s.encrypt(cookies)
		if err != nil {
			return fmt.Errorf("加密cookies失败: %w", err)
		}
		updates["cookies"] = encrypted
	}

	if token, ok := updates["access_token"].(string); ok && token != "" {
		encrypted, err := s.encrypt(token)
		if err != nil {
			return fmt.Errorf("加密access_token失败: %w", err)
		}
		updates["access_token"] = encrypted
	}

	if token, ok := updates["refresh_token"].(string); ok && token != "" {
		encrypted, err := s.encrypt(token)
		if err != nil {
			return fmt.Errorf("加密refresh_token失败: %w", err)
		}
		updates["refresh_token"] = encrypted
	}

	return s.DB.Model(binding).Updates(updates).Error
}

// GetUserBindings 获取用户的所有绑定
func (s *BilibiliAccountService) GetUserBindings(userID string) ([]model.AccountBinding, error) {
	var bindings []model.AccountBinding
	err := s.DB.Where("user_id = ? AND status = ?", userID, model.BindingStatusBound).
		Order("is_primary DESC, created_at DESC").
		Find(&bindings).Error
	if err != nil {
		s.logger.Errorf("获取用户绑定列表失败: %v", err)
		return nil, fmt.Errorf("获取用户绑定列表失败: %w", err)
	}
	return bindings, nil
}

// GetPrimaryBinding 获取用户的主绑定（针对特定平台）
func (s *BilibiliAccountService) GetPrimaryBinding(userID string, platform model.Platform) (*model.AccountBinding, error) {
	var binding model.AccountBinding
	err := s.DB.Where("user_id = ? AND platform = ? AND is_primary = ? AND status = ?",
		userID, platform, true, model.BindingStatusBound).
		First(&binding).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		s.logger.Errorf("获取主绑定失败: %v", err)
		return nil, fmt.Errorf("获取主绑定失败: %w", err)
	}
	return &binding, nil
}

// SetPrimaryBinding 设置主绑定
func (s *BilibiliAccountService) SetPrimaryBinding(userID string, platform model.Platform, platformUID string) error {
	// 开启事务
	tx := s.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 先将该用户该平台的所有绑定的主标志设为false
	if err := tx.Model(&model.AccountBinding{}).
		Where("user_id = ? AND platform = ?", userID, platform).
		Update("is_primary", false).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("清除主绑定标志失败: %w", err)
	}

	// 设置指定绑定为主绑定
	if err := tx.Model(&model.AccountBinding{}).
		Where("user_id = ? AND platform = ? AND platform_uid = ?", userID, platform, platformUID).
		Update("is_primary", true).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("设置主绑定失败: %w", err)
	}

	return tx.Commit().Error
}

// DeleteBinding 删除绑定（软删除）
func (s *BilibiliAccountService) DeleteBinding(bindingID uint) error {
	return s.DB.Delete(&model.AccountBinding{}, bindingID).Error
}

// GetDecryptedCookies 获取解密后的cookies
func (s *BilibiliAccountService) GetDecryptedCookies(binding *model.AccountBinding) (string, error) {
	return s.decrypt(binding.Cookies)
}

// GetDecryptedTokens 获取解密后的tokens
func (s *BilibiliAccountService) GetDecryptedTokens(binding *model.AccountBinding) (accessToken, refreshToken string, err error) {
	accessToken, err = s.decrypt(binding.AccessToken)
	if err != nil {
		return "", "", err
	}

	refreshToken, err = s.decrypt(binding.RefreshToken)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// UpdateLastUsed 更新最后使用时间
func (s *BilibiliAccountService) UpdateLastUsed(bindingID uint) error {
	now := time.Now()
	return s.DB.Model(&model.AccountBinding{}).
		Where("id = ?", bindingID).
		Update("last_used_at", &now).Error
}

// GetBinding 根据ID获取绑定
func (s *BilibiliAccountService) GetBinding(bindingID uint) (*model.AccountBinding, error) {
	var binding model.AccountBinding
	err := s.DB.First(&binding, bindingID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &binding, nil
}

// GetBindingByPlatform 根据平台和平台UID获取绑定
func (s *BilibiliAccountService) GetBindingByPlatform(userID string, platform model.Platform, platformUID string) (*model.AccountBinding, error) {
	var binding model.AccountBinding
	err := s.DB.Where("user_id = ? AND platform = ? AND platform_uid = ?",
		userID, platform, platformUID).First(&binding).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &binding, nil
}

// GetLatestBinding 获取最近更新的有效绑定（用于系统自动任务，不区分用户）
func (s *BilibiliAccountService) GetLatestBinding(platform model.Platform) (*model.AccountBinding, error) {
	var binding model.AccountBinding
	err := s.DB.Where("platform = ? AND status = ?", platform, model.BindingStatusBound).
		Order("updated_at DESC").
		First(&binding).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("获取最新绑定失败: %w", err)
	}
	return &binding, nil
}
