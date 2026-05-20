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
        <v-btn
          :prepend-icon="sortAsc ? 'mdi-sort-clockwise-ascending' : 'mdi-sort-clockwise-descending'"
          size="small"
          variant="text"
          @click="sortAsc = !sortAsc"
        >
          {{ sortAsc ? '正序' : '倒序' }}
        </v-btn>
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
                      <template v-if="record.elecConsumption > 0">充值</template>
                      <template v-else-if="record.elecConsumption === 0">≈0 度</template>
                      <template v-else>{{ Math.abs(record.elecConsumption).toFixed(1) }} 度</template>
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
const sortAsc = ref(false) // false = 默认倒序（最新在前）

// 附加日消耗量到每条记录（按日期正序：旧→新）
const recordsWithConsumption = computed(() => {
  // 按日期正序排列（由记录条数决定，不依赖 records 本身顺序）
  const n = records.value.length
  if (n === 0) return []

  // 复制并按日期正序（老→新）
  const sorted = [...records.value].sort((a, b) =>
    a.record_date < b.record_date ? -1 : a.record_date > b.record_date ? 1 : 0
  )

  // 若为倒序则反转（最新在前），始终保证 computed 结果直接用于 v-for
  const logs = sortAsc.value ? sorted : sorted.reverse()

  const today = new Date().toISOString().slice(0, 10) // 'YYYY-MM-DD'

  // 从第二条开始算日消耗，第一条无前驱数据
  const result = logs.map((log, i) => {
    // prev 始终取 sorted 中的前驱日期（正序时用 sorted[i-1]，倒序时用 sorted[i+1]）
    let prev = null
    if (sortAsc.value) {
      // 正序：sorted[i-1] 为前一天
      prev = i > 0 ? sorted[i - 1] : null
    } else {
      // 倒序：sorted[i+1] 为前一天（因为 reverse 后索引越大日期越早）
      prev = i + 1 < sorted.length ? sorted[i + 1] : null
    }

    let elecConsumption = null
    if (prev && prev.remaining_kwh != null && log.remaining_kwh != null) {
      // 今天余额 - 昨天余额 = 变化量（正=充值增加，负=消耗减少）
      const d = parseFloat(log.remaining_kwh) - parseFloat(prev.remaining_kwh)
      elecConsumption = Math.round(d * 10) / 10
    }

    let waterConsumption = null
    if (prev && prev.remaining_water != null && log.remaining_water != null) {
      // 已用水量昨天 - 今天 = 消耗（abs总为正）
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
