<template>
  <div>
    <!-- ====== Token 输入（未鉴权时，居中显示）====== -->
    <v-row v-if="!adminAuthed" justify="center">
      <v-col cols="12" sm="8" md="5" lg="4">
        <v-card class="pa-5" elevation="0" :style="{ border: '1px solid rgba(var(--v-theme-outline),0.2)' }">
          <div class="text-center mb-4">
            <v-avatar color="warning" size="48" class="mb-3">
              <v-icon color="white" size="24">mdi-shield-lock</v-icon>
            </v-avatar>
            <div class="text-subtitle-1 font-weight-bold">管理后台</div>
            <div class="text-caption text-medium-emphasis mt-1">请输入管理员 Token 继续</div>
          </div>
          <v-text-field
            v-model="tokenInput"
            label="Admin Token"
            type="password"
            prepend-inner-icon="mdi-key-outline"
            :append-inner-icon="showToken ? 'mdi-eye-off' : 'mdi-eye'"
            :type="showToken ? 'text' : 'password'"
            density="comfortable"
            class="mb-3"
            @click:append-inner="showToken = !showToken"
            @keydown.enter="submitToken"
          />
          <v-btn color="primary" :loading="authLoading" block @click="submitToken">
            <v-icon start>mdi-login</v-icon>
            验证并进入
          </v-btn>
          <v-alert v-if="authError" type="error" variant="tonal" density="compact" class="mt-3" rounded="lg">
            <v-icon size="14" class="mr-1">mdi-alert</v-icon>{{ authError }}
          </v-alert>
        </v-card>
      </v-col>
    </v-row>

    <!-- ====== 管理内容（已鉴权后） ====== -->
    <template v-if="adminAuthed">

      <!-- 同步状态卡片 -->
      <v-card class="mb-4 pa-4" elevation="0" :style="{ border: '1px solid rgba(var(--v-theme-outline),0.2)' }">
        <div class="d-flex align-center mb-3">
          <v-icon color="primary" class="mr-2" size="20">mdi-database-sync</v-icon>
          <span class="text-subtitle-2 font-weight-bold">当前同步状态</span>
          <v-spacer />
          <v-btn icon="mdi-refresh" size="small" variant="text" :loading="loadingStatus" @click="fetchSyncStatus" />
        </div>

        <v-row dense>
          <v-col cols="6" sm="3">
            <div class="stat-block">
              <div class="stat-label">有效房间数</div>
              <div class="stat-value text-primary">{{ syncStatus.total_rooms ?? '—' }}</div>
            </div>
          </v-col>
          <v-col cols="6" sm="3">
            <div class="stat-block">
              <div class="stat-label">数据状态</div>
              <v-chip :color="syncStatus.initialized ? 'success' : 'warning'" size="small" variant="tonal" class="mt-1">
                {{ syncStatus.initialized ? '已初始化' : '待初始化' }}
              </v-chip>
            </div>
          </v-col>
          <v-col cols="6" sm="3">
            <div class="stat-block">
              <div class="stat-label">上次同步</div>
              <div class="stat-value">{{ formatDateTime(syncStatus.last_sync_at) }}</div>
            </div>
          </v-col>
          <v-col cols="6" sm="3">
            <div class="stat-block">
              <div class="stat-label">下次定时同步</div>
              <div class="stat-value">{{ formatDateTime(syncStatus.next_sync_at) }}</div>
            </div>
          </v-col>
        </v-row>

        <v-alert
          v-if="syncStatus.is_running"
          type="info"
          variant="tonal"
          class="mt-3"
          density="compact"
        >
          <template #prepend>
            <v-progress-circular indeterminate size="18" width="2" color="info" />
          </template>
          同步正在进行中，请耐心等待（通常需要 3~5 分钟）...
        </v-alert>
      </v-card>

      <!-- 手动触发 -->
      <v-card class="mb-4 pa-4" elevation="0" :style="{ border: '1px solid rgba(var(--v-theme-outline),0.2)' }">
        <div class="text-subtitle-2 font-weight-bold mb-2">
          <v-icon color="warning" class="mr-1" size="18">mdi-lightning-bolt</v-icon>
          手动触发全量同步
        </div>
        <div class="text-body-2 text-medium-emphasis mb-3">
          强制重新从官网抓取所有楼栋、楼层、房间数据，绕过 30 天定时限制。<br/>
          操作会在后台异步执行，期间服务正常运行，约 3~5 分钟完成。
        </div>
        <v-btn
          color="warning"
          :loading="triggeringSync"
          :disabled="syncStatus.is_running"
          @click="triggerSync"
        >
          <v-icon start>mdi-database-refresh</v-icon>
          立即同步房间列表
        </v-btn>
      </v-card>

    </template>
  </div>
</template>

<script setup>
import { ref, reactive, inject, onMounted } from 'vue'
import { adminAPI } from '@/api/index.js'

const adminAuthed = inject('adminAuthed')
const notify = inject('notify')

// ---- 鉴权 ----
const tokenInput  = ref('')
const showToken   = ref(false)
const authLoading = ref(false)
const authError   = ref('')

const submitToken = async () => {
  if (!tokenInput.value.trim()) {
    authError.value = 'Token 不能为空'
    return
  }
  authLoading.value = true
  authError.value = ''
  const prev = localStorage.getItem('eq_admin_token')
  localStorage.setItem('eq_admin_token', tokenInput.value.trim())
  try {
    await adminAPI.getSyncStatus()
    adminAuthed.value = true
    notify('管理员验证成功', 'success')
    fetchSyncStatus()
  } catch (err) {
    localStorage.setItem('eq_admin_token', prev || '')
    authError.value = err.response?.status === 401
      ? 'Token 无效，请检查配置中的 internal_token'
      : '验证失败: ' + (err.response?.data?.msg || err.message)
  } finally {
    authLoading.value = false
  }
}

// ---- 同步状态（持久化存储）----
const SYNC_STATUS_KEY = 'eq_sync_status'
const syncStatus     = reactive(
  JSON.parse(localStorage.getItem(SYNC_STATUS_KEY) || '{}')
)
const loadingStatus  = ref(false)
const triggeringSync = ref(false)

const fetchSyncStatus = async () => {
  loadingStatus.value = true
  try {
    const res = await adminAPI.getSyncStatus()
    Object.assign(syncStatus, res.data.data)
    localStorage.setItem(SYNC_STATUS_KEY, JSON.stringify(syncStatus))
  } catch {
    notify('获取同步状态失败', 'error')
  } finally {
    loadingStatus.value = false
  }
}

const triggerSync = async () => {
  triggeringSync.value = true
  try {
    const res = await adminAPI.triggerSync()
    notify(res.data.msg || '同步已触发', 'success')
    setTimeout(fetchSyncStatus, 1000)
  } catch (err) {
    notify(err.response?.data?.msg || '触发失败', 'error')
  } finally {
    triggeringSync.value = false
  }
}

const formatDateTime = (iso) => {
  if (!iso) return '未知'
  const d = new Date(iso)
  return `${d.toLocaleDateString('zh-CN')} ${d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })}`
}

onMounted(() => {
  if (adminAuthed.value) fetchSyncStatus()
})
</script>

<style scoped>
.stat-block { padding: 8px 0; }
.stat-label { font-size: 12px; color: rgba(var(--v-theme-on-surface), 0.6); margin-bottom: 4px; }
.stat-value { font-size: 15px; font-weight: 600; }
</style>
