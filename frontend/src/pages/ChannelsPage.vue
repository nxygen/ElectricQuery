<template>
  <div>
    <!-- ====== 企业微信卡片 ====== -->
    <v-card class="mb-4 pa-5" elevation="0">
      <div class="d-flex align-center mb-4">
        <v-avatar color="green-lighten-4" size="44" class="mr-3">
          <v-icon color="green-darken-2" size="24">mdi-wechat</v-icon>
        </v-avatar>
        <div class="flex-grow-1">
          <div class="text-subtitle-1 font-weight-bold">企业微信机器人</div>
          <div class="text-caption text-medium-emphasis">
            通过群机器人 Webhook 接收电量告警和周报通知
          </div>
        </div>
        <v-chip
          :color="form.wechat_webhook ? 'success' : 'grey'"
          text-color="white"
          size="small"
          variant="tonal"
        >
          <v-icon start size="14">
            {{ form.wechat_webhook ? 'mdi-check' : 'mdi-close' }}
          </v-icon>
          {{ form.wechat_webhook ? '已配置' : '未配置' }}
        </v-chip>
      </div>

      <v-text-field
        v-model="form.wechat_webhook"
        label="Webhook URL"
        prepend-inner-icon="mdi-link-variant"
        placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."
        clearable
        class="mb-2"
        :rules="[rules.url]"
        hint="在企业微信群中添加「群机器人」，复制 Webhook 地址粘贴至此"
        persistent-hint
      />

      <v-alert
        type="info"
        variant="tonal"
        density="compact"
        rounded="lg"
        class="mb-2"
        icon="mdi-information-outline"
      >
        <div class="text-caption">
          添加机器人：在企业微信群 → 群设置 → 群机器人 → 添加机器人 → 复制 Webhook URL
        </div>
      </v-alert>
    </v-card>

    <!-- ====== 邮件通知卡片 ====== -->
    <v-card class="mb-4 pa-5" elevation="0">
      <div class="d-flex align-center mb-4">
        <v-avatar color="blue-lighten-4" size="44" class="mr-3">
          <v-icon color="blue-darken-2" size="24">mdi-email</v-icon>
        </v-avatar>
        <div class="flex-grow-1">
          <div class="text-subtitle-1 font-weight-bold">邮件通知</div>
          <div class="text-caption text-medium-emphasis">
            接收电量告警和每周用电报告
          </div>
        </div>
        <v-chip
          :color="form.email ? 'success' : 'grey'"
          text-color="white"
          size="small"
          variant="tonal"
        >
          <v-icon start size="14">
            {{ form.email ? 'mdi-check' : 'mdi-close' }}
          </v-icon>
          {{ form.email ? '已配置' : '未配置' }}
        </v-chip>
      </div>

      <v-text-field
        v-model="form.email"
        label="接收邮箱"
        prepend-inner-icon="mdi-at"
        placeholder="your.name@email.com"
        type="email"
        clearable
        class="mb-2"
        :rules="[rules.email]"
        hint="用于接收电量告警和周报，格式需为有效邮箱地址"
        persistent-hint
      />
    </v-card>

    <!-- ====== 保存 & 测试按钮 ====== -->
    <v-card class="pa-5" elevation="0">
      <!-- 测试通知结果 -->
      <v-expand-transition>
        <v-alert
          v-if="testResult !== null"
          :type="testResult ? 'success' : 'error'"
          variant="tonal"
          rounded="lg"
          class="mb-4"
          :icon="testResult ? 'mdi-check-circle' : 'mdi-alert-circle'"
        >
          {{ testResult ? '✅ 测试通知发送成功！请检查您的企业微信群/邮箱是否收到消息' : '❌ 测试通知发送失败，请检查配置是否正确' }}
        </v-alert>
      </v-expand-transition>

      <!-- 保存按钮 + 测试选项 -->
      <div class="d-flex align-center gap-3 mb-4">
        <v-btn
          color="primary"
          size="large"
          class="flex-grow-1"
          :loading="saving"
          @click="saveChannels(false)"
        >
          <v-icon start>mdi-content-save</v-icon>
          保存配置
        </v-btn>
      </div>

      <!-- 保存并发送测试 -->
      <div class="d-flex align-center gap-3">
        <v-checkbox
          v-model="sendTestAfterSave"
          label="保存后发送测试通知"
          color="primary"
          density="compact"
          hide-details
          class="test-checkbox"
        />
        <v-spacer />
        <v-btn
          variant="outlined"
          color="primary"
          size="large"
          :loading="testing"
          :disabled="!form.wechat_webhook && !form.email"
          @click="saveChannels(true)"
        >
          <v-icon start>mdi-paper-plane</v-icon>
          发送测试通知
        </v-btn>
      </div>

      <v-divider class="my-5" />

      <!-- 说明卡片 -->
      <v-card variant="tonal" color="primary-container" class="pa-4" elevation="0">
        <div class="d-flex align-center mb-3">
          <v-icon color="primary" size="18" class="mr-2">mdi-bell-ring</v-icon>
          <div class="text-subtitle-2 font-weight-bold text-primary">通知触发场景</div>
        </div>
        <v-list density="compact" bg-color="transparent">
          <v-list-item
            prepend-icon="mdi-lightning-bolt"
            title="电量低于 20 度时"
            subtitle="系统轮询发现电量不足，自动发送告警"
            class="px-0"
          />
          <v-list-item
            prepend-icon="mdi-calendar-week"
            title="每周一早 8:00"
            subtitle="自动发送过去 7 天的用电周报"
            class="px-0"
          />
          <v-list-item
            prepend-icon="mdi-flash"
            title="测试通知（立即）"
            subtitle="保存配置后可立即发送测试，确认渠道可用"
            class="px-0"
          />
        </v-list>
      </v-card>

      <!-- 隐私说明 -->
      <v-card variant="outlined" class="pa-4 mt-4" elevation="0">
        <div class="d-flex align-center mb-2">
          <v-icon color="medium-emphasis" size="18" class="mr-2">mdi-shield-check</v-icon>
          <div class="text-subtitle-2 font-weight-bold">隐私说明</div>
        </div>
        <div class="text-body-2 text-medium-emphasis">
          您的通知渠道凭证仅用于向您本人发送通知，绝不会用于其他用途。
          Webhook URL 以加密方式存储。
        </div>
      </v-card>
    </v-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, inject } from 'vue'
import { userAPI } from '@/api/index.js'

const notify = inject('notify')

const form  = reactive({ wechat_webhook: '', email: '' })
const saving     = ref(false)
const testing    = ref(false)
const sendTestAfterSave = ref(true)
const testResult = ref(null)

const rules = {
  url: v => {
    if (!v) return true
    return v.startsWith('https://qyapi.weixin.qq.com/') || 'Webhook URL 格式不正确'
  },
  email: v => {
    if (!v) return true
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) || '邮箱格式不正确'
  }
}

onMounted(async () => {
  try {
    const res = await userAPI.getChannel()
    const ch = res.data.data || {}
    form.wechat_webhook = ch.wechat_webhook || ''
    form.email          = ch.email          || ''
  } catch {}
})

// 保存渠道配置，可选触发测试通知
const saveChannels = async (forceTest = false) => {
  const doTest = forceTest || sendTestAfterSave.value
  const hasAnyChannel = form.wechat_webhook || form.email

  if (!hasAnyChannel) {
    notify('请至少填写一种通知渠道', 'warning')
    return
  }

  saving.value = true
  testResult.value = null
  try {
    await userAPI.updateChannel({
      wechat_webhook: String(form.wechat_webhook || ''),
      email: String(form.email || ''),
      test_channel: !!(doTest && hasAnyChannel),
    })
    notify(doTest ? '配置已保存，正在发送测试通知...' : '配置已保存')
  } catch (err) {
    notify(err.response?.data?.msg || '保存失败', 'error')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.test-checkbox {
  flex-shrink: 0;
}
</style>
