<template>
  <div class="history-page">
    <!-- ====== Token 输入（未登录时，居中显示）====== -->
    <v-row v-if="!isLoggedIn" justify="center">
      <v-col cols="12" sm="8" md="5" lg="4">
        <v-card class="pa-5" elevation="0" :style="{ border: '1px solid rgba(var(--v-theme-outline),0.2)' }">
          <div class="text-center">
            <v-icon color="primary" size="48" class="mb-3">mdi-history</v-icon>
            <div class="text-subtitle-1 font-weight-bold mb-2">历史记录</div>
            <div class="text-caption text-medium-emphasis mb-4">登录后可查看历史记录</div>
            <v-btn color="primary" block to="/login">去登录</v-btn>
          </div>
        </v-card>
      </v-col>
    </v-row>

    <!-- ====== 未绑定宿舍 ====== -->
    <v-row v-else-if="!userInfo?.dorm_room" justify="center">
      <v-col cols="12" sm="8" md="5" lg="4">
        <v-card class="pa-5" elevation="0" :style="{ border: '1px solid rgba(var(--v-theme-outline),0.2)' }">
          <div class="text-center">
            <v-icon color="warning" size="48" class="mb-3">mdi-home-exclamation</v-icon>
            <div class="text-subtitle-1 font-weight-bold mb-2">未绑定宿舍</div>
            <div class="text-caption text-medium-emphasis mb-4">请先在个人中心绑定宿舍号</div>
            <v-btn color="primary" variant="tonal" block to="/me">去绑定</v-btn>
          </div>
        </v-card>
      </v-col>
    </v-row>

    <!-- ====== 历史记录列表 ====== -->
    <template v-else>
      <!-- 头部信息 -->
      <div class="d-flex align-center mb-4">
        <v-icon color="primary" class="mr-2">mdi-history</v-icon>
        <span class="text-subtitle-1 font-weight-bold">历史记录</span>
        <v-spacer />
        <v-btn icon="mdi-refresh" size="small" variant="text" :loading="loading" @click="fetchRecords" />
      </div>

      <!-- 加载中 -->
      <div v-if="loading" class="text-center py-8">
        <v-progress-circular indeterminate color="primary" />
      </div>

      <!-- 空状态 -->
      <v-card v-else-if="records.length === 0" class="pa-6" elevation="0"
        :style="{ border: '1px solid rgba(var(--v-theme-outline),0.2)' }">
        <div class="text-center text-medium-emphasis">
          <v-icon size="48" class="mb-2">mdi-clipboard-text-outline</v-icon>
          <div>暂无历史记录</div>
          <div class="text-caption mt-1">系统将每隔一段时间自动记录用电数据</div>
        </div>
      </v-card>

      <!-- 记录列表 -->
      <div v-else class="records-list">
        <v-card
          v-for="(record, idx) in recordsWithConsumption"
          :key="record.id"
          class="mb-3 pa-4"
          elevation="0"
          :style="{ border: '1px solid rgba(var(--v-theme-outline),0.2)' }"
        >
          <div class="d-flex align-center justify-space-between mb-2">
            <div class="d-flex align-center">
              <v-icon size="16" class="mr-1 text-medium-emphasis">mdi-calendar</v-icon>
              <span class="text-body-2 font-weight-medium">{{ formatDate(record.record_date) }}</span>
            </div>
            <span v-if="record.isToday" class="text-caption text-primary">今日</span>
            <span v-else class="text-caption text-medium-emphasis">{{ formatTime(record.queried_at) }}</span>
          </div>

          <v-row dense>
            <!-- 电量消耗 -->
            <v-col cols="6">
              <div class="d-flex align-center">
                <v-icon size="18" color="primary" class="mr-2">mdi-lightning-bolt</v-icon>
                <div>
                  <div class="text-caption text-medium-emphasis">用电</div>
                  <div class="text-body-1 font-weight-bold text-primary">
                    <template v-if="record.elecConsumption !== null">
                      <template v-if="record.elecConsumption < 0">充值</template>
                      <template v-else>+{{ record.elecConsumption.toFixed(1) }} 度</template>
                    </template>
                    <template v-else>—</template>
                  </div>
                </div>
              </div>
            </v-col>
            <!-- 水量消耗 -->
            <v-col cols="6">
              <div class="d-flex align-center">
                <v-icon size="18" color="info" class="mr-2">mdi-water</v-icon>
                <div>
                  <div class="text-caption text-medium-emphasis">用水</div>
                  <div class="text-body-1 font-weight-bold text-info">
                    <template v-if="record.waterConsumption !== null">
                      {{ record.waterConsumption.toFixed(1) }} 吨
                    </template>
                    <template v-else>—</template>
                  </div>
                </div>
              </div>
            </v-col>
          </v-row>

          <div v-if="record.dorm_label" class="text-caption text-medium-emphasis mt-2">
            <v-icon size="12" class="mr-1">mdi-home</v-icon>
            {{ record.dorm_label }}
          </div>
        </v-card>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, inject, onMounted } from 'vue'
import { powerAPI } from '@/api/index.js'

const notify = inject('notify')
const userInfo = inject('userInfo')

const isLoggedIn = !!localStorage.getItem('eq_token')
const records = ref([])
const loading = ref(false)

// 附加日消耗量到每条记录（按日期正序：旧→新）
const recordsWithConsumption = computed(() => {
  const logs = [...records.value].reverse() // 旧→新
  if (logs.length === 0) return []

  const today = new Date().toISOString().slice(0, 10) // 'YYYY-MM-DD'

  // 从第二条开始算日消耗，第一条无前驱数据
  const result = logs.map((log, i) => {
    const prev = i > 0 ? logs[i - 1] : null

    let elecConsumption = null
    if (prev && prev.remaining_kwh != null && log.remaining_kwh != null) {
      const d = parseFloat(prev.remaining_kwh) - parseFloat(log.remaining_kwh)
      elecConsumption = Math.round(d * 10) / 10
    }

    let waterConsumption = null
    if (prev && prev.remaining_water != null && log.remaining_water != null) {
      // remaining_water 为负数（已用水量），昨天 - 今天 = 今日消耗（正数）
      const d = Math.abs(parseFloat(prev.remaining_water) - parseFloat(log.remaining_water))
      waterConsumption = Math.round(d * 10) / 10
    }

    return {
      ...log,
      isToday: log.record_date === today,
      elecConsumption,
      waterConsumption,
    }
  })

  return result
})

const fetchRecords = async () => {
  loading.value = true
  try {
    const res = await powerAPI.records(90)
    records.value = res.data?.data || []
  } catch {
    notify('加载历史记录失败', 'error')
  } finally {
    loading.value = false
  }
}

const formatDate = (iso) => {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', weekday: 'short' })
}

const formatTime = (iso) => {
  if (!iso) return ''
  const d = new Date(iso)
  return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

onMounted(() => {
  if (isLoggedIn && userInfo?.dorm_room) fetchRecords()
})
</script>

<style scoped>
.history-page {
  max-width: 700px;
  margin: 0 auto;
}
</style>
