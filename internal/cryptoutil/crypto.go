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
// 若解密失败（非加密数据或密钥错误），返回空字符串，不报错
// 这用于向后兼容：旧版本存储的明文 secret 无法被解密，会静默返回空
func Decrypt(ciphertextB64, kek string) (string, error) {
	if ciphertextB64 == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		// 不是有效的 base64（旧数据）→ 静默返回原值，调用方负责判断
		return "", nil
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
		return "", nil
	}
	nonce, ct := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		// 密钥不匹配或数据损坏（旧数据）
		return "", nil
	}
	return string(plaintext), nil
}
