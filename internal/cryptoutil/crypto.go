// Package cryptoutil 提供加解密工具函数
// 使用 AES-256-GCM 对称加密，密钥由配置中的 jwt_secret 派生（KEK 模式）
package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// deriveKey 从任意长度的输入派生 32 字节 AES-256 密钥
// 注意：当前与 JWT 签名共用同一 secret，违反密钥分离原则。
// 若需分离，需配合数据迁移将现有密文用新 key 重新加密。
func deriveKey(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

// Encrypt 使用 AES-256-GCM 加密明文，返回 base64(ciphertext) 格式
// 格式：base64(nonce[12] || ciphertext || tag[16])
func Encrypt(plaintext, kek string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key := deriveKey(kek)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建加密块失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密 base64 密文，kek 必须与 Encrypt 时使用的相同
// 返回值：
//   - ("", nil)：输入为空
//   - ("", error)：解密失败（非 base64、密钥不匹配、数据损坏）
//   - (plaintext, nil)：解密成功
func Decrypt(ciphertextB64, kek string) (string, error) {
	if ciphertextB64 == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("无效的 base64 数据: %w", err)
	}
	key := deriveKey(kek)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("创建解密块失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("密文长度不足")
	}
	nonce, ct := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败（密钥不匹配或数据损坏）: %w", err)
	}
	return string(plaintext), nil
}
