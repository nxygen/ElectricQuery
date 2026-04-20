/**
 * Token 生成工具（前端版本）
 * 
 * 基于 HMAC-SHA256 生成短期 token，格式: "<timestamp>.<hmac_hex>"
 * 需要与后端共享 secret（从 vite.config 注入或环境变量读取）
 */

/**
 * 将字符串转为 UTF-8 字节数组
 */
function str2bytes(str) {
  const encoder = new TextEncoder()
  return encoder.encode(str)
}

/**
 * 将 ArrayBuffer 转为十六进制字符串
 */
function buf2hex(buffer) {
  return Array.from(new Uint8Array(buffer))
    .map(b => b.toString(16).padStart(2, '0'))
    .join('')
}

/**
 * 生成 HMAC-SHA256 签名（使用 Web Crypto API）
 * @param {string} secret - 密钥
 * @param {string} message - 消息
 * @returns {Promise<string>} 十六进制签名
 */
async function hmacSha256(secret, message) {
  const keyData = str2bytes(secret)
  const msgData = str2bytes(message)
  
  const key = await crypto.subtle.importKey(
    'raw',
    keyData,
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign']
  )
  
  const signature = await crypto.subtle.sign('HMAC', key, msgData)
  return buf2hex(signature)
}

/**
 * 生成短期 token
 * @param {string} secret - 后端与客户端共享的密钥
 * @returns {Promise<string>} token 格式: "<timestamp>.<hmac_hex>"
 */
export async function generateToken(secret) {
  if (!secret) {
    throw new Error('secret is required')
  }
  
  const timestamp = Math.floor(Date.now() / 1000)
  const signature = await hmacSha256(secret, String(timestamp))
  
  return `${timestamp}.${signature}`
}
