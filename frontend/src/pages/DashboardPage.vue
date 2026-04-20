<template>
  <div>
    <!-- ====== 水电状态概览 ====== -->
    <v-row class="mb-5">
      <!-- 电量卡片 -->
      <v-col cols="12" sm="6" lg="4">
        <v-card
          class="status-card pa-4 h-100"
          :class="powerCardClass"
          elevation="0"
        >
          <div class="d-flex align-center justify-space-between mb-3">
            <div class="d-flex align-center">
              <v-avatar
                :color="powerCardClass === 'card-warning' ? 'warning-container' : 'primary-container'"
                size="40"
                class="mr-3"
              >
                <v-icon
                  :color="powerCardClass === 'card-warning' ? 'warning' : 'primary'"
                  size="22"
                >
                  mdi-lightning-bolt
                </v-icon>
              </v-avatar>
              <div>
                <div class="text-overline opacity-60">剩余电量</div>
                <div class="text-h4 font-weight-bold">
                  {{ currentPower !== null ? currentPower.toFixed(1) : '—' }}
                  <span v-if="currentPower !== null" class="text-body-1 font-weight-normal">度</span>
                </div>
              </div>
            </div>
            <!-- 状态标签 -->
            <v-chip
              :color="powerCardClass === 'card-warning' ? 'warning' : 'success'"
              text-color="white"
              size="small"
              variant="tonal"
            >
              {{ powerStatusText }}
            </v-chip>
          </div>

          <!-- 进度条可视化 -->
          <div v-if="currentPower !== null" class="mb-3">
            <div class="d-flex justify-space-between text-caption text-medium-emphasis mb-1">
              <span>剩余比例</span>
              <span>{{ powerPercent }}%</span>
            </div>
            <v-progress-linear
              :model-value="powerPercent"
              :color="powerCardClass === 'card-warning' ? 'warning' : 'primary'"
              height="8"
              rounded
              bg-color="surface-variant"
            />
          </div>

          <div class="text-caption text-medium-emphasis mb-3">
            <v-icon size="12" class="mr-1">mdi-home</v-icon>
            {{ formattedDorm || '未绑定宿舍' }}
          </div>

          <div class="d-flex gap-2">
            <v-btn
              variant="tonal"
              color="primary"
              size="small"
              :loading="queryingPower"
              :disabled="!dormRoom"
              @click="queryNow"
            >
              <v-icon start size="16">mdi-refresh</v-icon>
              立即查询
            </v-btn>
            <!-- 自动刷新开关 -->
            <v-btn
              :variant="autoRefresh ? 'flat' : 'outlined'"
              :color="autoRefresh ? 'primary' : undefined"
              size="small"
              @click="autoRefresh = !autoRefresh"
            >
              <v-icon start size="16">mdi-timer</v-icon>
              {{ autoRefresh ? `${refreshCountdown}s` : '自动刷新' }}
            </v-btn>
          </div>
        </v-card>
      </v-col>

      <!-- 水量卡片 -->
      <v-col cols="12" sm="6" lg="4">
        <v-card
          class="status-card pa-4 h-100"
          :class="waterCardClass"
          elevation="0"
        >
          <div class="d-flex align-center justify-space-between mb-3">
            <div class="d-flex align-center">
              <v-avatar
                :color="waterCardClass === 'card-warning' ? 'warning-container' : 'info-container'"
                size="40"
                class="mr-3"
              >
                <v-icon
                  :color="waterCardClass === 'card-warning' ? 'warning' : 'info'"
                  size="22"
                >
                  mdi-water
                </v-icon>
              </v-avatar>
              <div>
                <div class="text-overline opacity-60">历史累计消耗</div>
                <div class="text-h4 font-weight-bold">
                  {{ totalWaterConsumed !== null ? totalWaterConsumed.toFixed(1) : '—' }}
                  <span v-if="totalWaterConsumed !== null" class="text-body-1 font-weight-normal">吨</span>
                </div>
              </div>
            </div>
            <v-chip
              :color="waterCardClass === 'card-warning' ? 'warning' : 'success'"
              text-color="white"
              size="small"
              variant="tonal"
            >
              {{ waterStatusText }}
            </v-chip>
          </div>

          <div v-if="totalWaterConsumed !== null" class="mb-3">
            <div class="d-flex justify-space-between text-caption text-medium-emphasis mb-1">
              <span>累计消耗</span>
              <span>{{ waterPercent }}%</span>
            </div>
            <v-progress-linear
              :model-value="waterPercent"
              :color="waterCardClass === 'card-warning' ? 'warning' : 'info'"
              height="8"
              rounded
              bg-color="surface-variant"
            />
          </div>
          <div v-else class="mb-3 text-caption text-medium-emphasis">
            暂无历史数据，点击刷新获取
          </div>

          <div class="text-caption text-medium-emphasis mb-3">
            <v-icon size="12" class="mr-1">mdi-home</v-icon>
            {{ formattedDorm || '未绑定宿舍' }}
          </div>

          <v-btn
            variant="tonal"
            color="info"
            size="small"
            :loading="queryingWater"
            :disabled="!dormRoom"
            @click="queryWater"
          >
            <v-icon start size="16">mdi-refresh</v-icon>
            刷新水量
          </v-btn>
        </v-card>
      </v-col>

      <!-- 快捷操作卡片 -->
      <v-col cols="12" lg="4">
        <v-card class="pa-4 h-100" elevation="0">
          <div class="text-subtitle-2 font-weight-bold mb-3">
            <v-icon class="mr-1 text-primary" size="16">mdi-lightning-bolt</v-icon>
            快捷入口
          </div>
          <v-list density="compact" nav>
            <v-list-item
              prepend-icon="mdi-account-edit"
              title="绑定宿舍信息"
              subtitle="楼栋、房间号、班级"
              to="/profile"
              rounded="lg"
              class="mb-1"
            />
            <v-list-item
              prepend-icon="mdi-bell-cog"
              title="配置通知渠道"
              subtitle="企业微信 / 邮件推送"
              to="/channels"
              rounded="lg"
            />
          </v-list>
          <v-divider class="my-3" />
          <div class="text-caption text-medium-emphasis">
            <v-icon size="12" class="mr-1">mdi-clock-outline</v-icon>
            最后查询：{{ lastQueryTime || '尚未查询' }}
          </div>
        </v-card>
      </v-col>
    </v-row>

    <!-- ====== 历史电量趋势图 ====== -->
    <v-card class="pa-5 mb-5" elevation="0">
      <div class="d-flex align-center justify-space-between mb-4">
        <div class="text-subtitle-1 font-weight-bold">
          <v-icon class="mr-1 text-primary" size="18">mdi-chart-line</v-icon>
          水电趋势（近 {{ historyLogs.length }} 天）
        </div>
        <div class="d-flex gap-2">
          <v-chip
            :color="powerCardClass === 'card-warning' ? 'warning' : 'primary'"
            text-color="white"
            size="small"
            variant="tonal"
          >
            告警阈值：20 度
          </v-chip>
        </div>
      </div>

      <!-- 趋势迷你图（文本表格形式） -->
      <v-table v-if="historyLogs.length > 0" density="comfortable">
        <thead>
          <tr>
            <th class="text-left" rowspan="2">日期</th>
            <th class="text-center" colspan="3">
              <v-icon size="14" class="mr-1 text-warning">mdi-lightning-bolt</v-icon>电量
            </th>
            <th v-if="hasWaterHistory" class="text-center" colspan="3">
              <v-icon size="14" class="mr-1 text-info">mdi-water</v-icon>水量
            </th>
          </tr>
          <tr>
            <th class="text-right text-caption">剩余（度）</th>
            <th class="text-right text-caption">当日消耗</th>
            <th class="text-right text-caption">趋势</th>
            <th v-if="hasWaterHistory" class="text-right text-caption">剩余（吨）</th>
            <th v-if="hasWaterHistory" class="text-right text-caption">当日变化</th>
            <th v-if="hasWaterHistory" class="text-right text-caption">趋势</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(item, i) in historyLogs.slice(0, 7)"
            :key="item.ID"
            :class="i === 0 ? 'bg-surface-variant font-weight-medium' : ''"
          >
            <td>{{ item.record_date }}</td>
            <!-- 电量列 -->
            <td class="text-right">{{ parseFloat(item.remaining_kwh).toFixed(2) }}</td>
            <td class="text-right">
              <v-chip
                v-if="i < historyLogs.length - 1"
                :color="getDeltaColor(historyLogs, i)"
                size="x-small"
                variant="tonal"
              >
                {{ getDelta(historyLogs, i) }}
              </v-chip>
              <span v-else class="text-medium-emphasis text-caption">—</span>
            </td>
            <td class="text-right">
              <v-icon
                :color="getTrendColor(historyLogs, i)"
                size="16"
              >
                {{ getTrendIcon(historyLogs, i) }}
              </v-icon>
            </td>
            <!-- 水量列（仅 C13/C14 楼有数据） -->
            <td v-if="hasWaterHistory" class="text-right">
              {{ item.remaining_water ? Math.abs(parseFloat(item.remaining_water)).toFixed(2) : '—' }}
            </td>
            <td v-if="hasWaterHistory" class="text-right">
              <v-chip
                v-if="i < historyLogs.length - 1 && item.remaining_water"
                :color="getWaterDeltaColor(historyLogs, i)"
                size="x-small"
                variant="tonal"
              >
                {{ getWaterDelta(historyLogs, i) }}
              </v-chip>
              <span v-else class="text-medium-emphasis text-caption">—</span>
            </td>
            <td v-if="hasWaterHistory" class="text-right">
              <v-icon
                :color="getWaterTrendColor(historyLogs, i)"
                size="16"
              >
                {{ getWaterTrendIcon(historyLogs, i) }}
              </v-icon>
            </td>
          </tr>
        </tbody>
      </v-table>

      <div v-else-if="!dormRoom" class="text-center py-8 text-medium-emphasis">
        <v-icon size="48" color="surface-variant" class="mb-2">mdi-home-off-outline</v-icon>
        <div class="text-body-1 mb-1">尚未绑定宿舍</div>
        <v-btn color="primary" variant="tonal" size="small" to="/profile">
          <v-icon start>mdi-account-edit</v-icon>
          前往绑定
        </v-btn>
      </div>

      <div v-else-if="loadingHistory" class="text-center py-8">
        <v-progress-circular indeterminate color="primary" />
        <div class="text-caption text-medium-emphasis mt-2">加载历史记录...</div>
      </div>

      <div v-else class="text-center py-8 text-medium-emphasis">
        <v-icon size="48" color="surface-variant" class="mb-2">mdi-chart-line-variant</v-icon>
        <div class="text-body-1 mb-1">暂无历史记录</div>
        <v-btn color="primary" variant="tonal" size="small" :loading="queryingPower" @click="queryNow">
          <v-icon start>mdi-refresh</v-icon>
          立即查询获取数据
        </v-btn>
      </div>
    </v-card>

    <!-- ====== 绑定提示 Banner（未绑定宿舍时显示）====== -->
    <v-alert
      v-if="!dormRoom"
      type="warning"
      variant="tonal"
      rounded="lg"
      class="mb-4"
      icon="mdi-home-alert"
    >
      <div class="text-body-2">
        您尚未绑定宿舍信息，
        <router-link to="/profile" class="text-primary font-weight-bold">立即前往绑定</router-link>
        后即可使用电量查询和告警通知功能。
      </div>
    </v-alert>

    <!-- ====== 未绑定学号提示 ====== -->
    <v-alert
      v-if="!hasStudentId"
      type="info"
      variant="tonal"
      rounded="lg"
      class="mb-4"
      icon="mdi-account-alert"
    >
      <div class="text-body-2">
        您尚未绑定学号，
        <router-link to="/profile" class="text-primary font-weight-bold">前往绑定</router-link>
        学号将作为您的登录账号。
      </div>
    </v-alert>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, inject, watch } from 'vue'
import { userAPI, powerAPI } from '@/api/index.js'

const notify = inject('notify')

const dormRoom      = ref('')
const currentPower  = ref(null)
const currentWater  = ref(null)  // 历史累计已用水量（吨）
const waterRawF     = ref(null)  // 计算用：原始剩余水量（可正可负，负=透支）
const waterDormRoom = ref('')

// 宿舍号拆分展示：140328 → C14 3楼 28号房间
const formattedDorm = computed(() => {
  const d = dormRoom.value || ''
  if (!d) return ''
  if (d.includes('-') || d.includes('C')) return d  // 已是标准格式如 C14-328
  // 纯数字格式：140328 → C14 / 3楼 / 28号房间
  // 前2位=楼号，中间2位=楼层，后2位=房间
  if (d.length >= 6) {
    const b = d.slice(0, 2)    // 14
    const f = d.slice(2, 4)    // 03
    const r = d.slice(4, 6)    // 28
    return `C${b} ${parseInt(f)}楼 ${parseInt(r)}号房间`
  }
  return d
})
const historyLogs    = ref([])
const queryingPower  = ref(false)
const queryingWater  = ref(false)
const loadingHistory = ref(false)
const lastQueryTime = ref('')

// 自动刷新
const autoRefresh    = ref(false)
const refreshCountdown = ref(30)
let   refreshTimer   = null
let   countdownTimer = null

// 电量阈值
const POWER_THRESHOLD = 20
const hasStudentId = ref(false)

// 电量状态
const powerCardClass = computed(() => {
  if (currentPower.value === null) return ''
  return currentPower.value < POWER_THRESHOLD ? 'card-warning' : 'card-ok'
})
const powerStatusText = computed(() => {
  if (currentPower.value === null) return '未查询'
  return currentPower.value < POWER_THRESHOLD ? '⚠️ 电量不足' : '✅ 电量充足'
})
const powerPercent = computed(() => {
  if (currentPower.value === null) return 0
  return Math.min(100, Math.round((currentPower.value / 100) * 100))
})

// 水量状态
const waterCardClass = computed(() => {
  if (totalWaterConsumed.value === null) return ''
  // 历史累计消耗超过 50 吨视为偏高（参考值）
  return totalWaterConsumed.value > 50 ? 'card-warning' : 'card-ok'
})
const waterStatusText = computed(() => {
  if (totalWaterConsumed.value === null) return '暂无数据'
  return totalWaterConsumed.value > 50 ? '⚠️ 消耗偏高' : '✅ 正常'
})
const waterPercent = computed(() => {
  if (totalWaterConsumed.value === null) return 0
  // 以 100 吨为参考满量，超过 100 吨按 100% 显示
  return Math.min(100, Math.round((totalWaterConsumed.value / 100) * 100))
})

// 自动刷新逻辑
watch(autoRefresh, (val) => {
  if (val) {
    refreshCountdown.value = 30
    countdownTimer = setInterval(() => {
      refreshCountdown.value--
      if (refreshCountdown.value <= 0) {
        refreshCountdown.value = 30
        queryNow()
      }
    }, 1000)
  } else {
    clearInterval(countdownTimer)
    refreshCountdown.value = 30
  }
})

// 查询电量（仅更新电量，不影响水量）
const queryNow = async () => {
  queryingPower.value = true
  try {
    const res = await powerAPI.query()
    const data = res.data.data
    currentPower.value = parseFloat(data?.remaining_kwh)
    lastQueryTime.value = new Date().toLocaleTimeString('zh-CN')
    notify(`查询成功，剩余 ${currentPower.value.toFixed(1)} 度`)
    await loadHistory()
  } catch (err) {
    notify(err.response?.data?.msg || '查询失败', 'error')
  } finally {
    queryingPower.value = false
  }
}

// 查询水费（独立刷新，重新拉取历史数据，totalWaterConsumed 由 historyLogs 驱动）
const queryWater = async () => {
  queryingWater.value = true
  try {
    const res = await powerAPI.queryWater(waterDormRoom.value || dormRoom.value)
    notify('水量已更新')
    await loadHistory()
  } catch (err) {
    const msg = err.response?.data?.msg || ''
    if (msg.includes('页面结构') || err.response?.status === 500) {
      notify('该宿舍暂无水费记录', 'warning')
    } else {
      notify(msg || '水费查询失败', 'error')
    }
  } finally {
    queryingWater.value = false
  }
}

// 加载历史记录
const loadHistory = async () => {
  if (!dormRoom.value) return
  loadingHistory.value = true
  try {
    const res = await powerAPI.history(14)
    historyLogs.value = res.data.data || []
    if (historyLogs.value.length > 0) {
      if (currentPower.value === null) {
        currentPower.value = parseFloat(historyLogs.value[0].remaining_kwh)
      }
    }
  } catch {}
  loadingHistory.value = false
}

// 计算每日消耗差值
const getDelta = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_kwh)
  const prev = parseFloat(logs[i + 1]?.remaining_kwh || curr)
  const d = curr - prev
  return d >= 0 ? `+${d.toFixed(2)}` : d.toFixed(2)
}
const getDeltaColor = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_kwh)
  const prev = parseFloat(logs[i + 1]?.remaining_kwh || curr)
  return curr < prev ? 'error' : 'success'
}
const getTrendColor = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_kwh)
  const prev = parseFloat(logs[i + 1]?.remaining_kwh || curr)
  if (i === 0) return curr < POWER_THRESHOLD ? 'warning' : 'success'
  return curr <= prev ? 'error' : 'success'
}
const getTrendIcon = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_kwh)
  const prev = parseFloat(logs[i + 1]?.remaining_kwh || curr)
  if (i === 0) return curr < POWER_THRESHOLD ? 'mdi-alert-circle' : 'mdi-check-circle'
  return curr <= prev ? 'mdi-trending-down' : 'mdi-trending-up'
}

// 水量历史：是否存在水量数据（C13/C14 楼才有）
const hasWaterHistory = computed(() => {
  return historyLogs.value.some(item => item.remaining_water && item.remaining_water.trim() !== '')
})

// 历史累计消耗：从所有历史记录累加每日消耗量
const totalWaterConsumed = computed(() => {
  const logs = historyLogs.value
  if (logs.length < 2) {
    // 不足两天数据：直接用 abs(latest) 作为参考值
    if (logs.length === 1 && logs[0].remaining_water) {
      return Math.abs(parseFloat(logs[0].remaining_water))
    }
    return null
  }
  // 从旧到新遍历，累加每日减少量（remaining 越来越负 = 消耗越来越多）
  let total = 0
  for (let i = logs.length - 1; i > 0; i--) {
    const curr = parseFloat(logs[i].remaining_water || 0)
    const prev = parseFloat(logs[i - 1].remaining_water || curr)
    const daily = prev - curr  // 例如 -50 - (-60) = 10（当天消耗10吨）
    if (daily > 0) total += daily
  }
  return Math.round(total * 100) / 100
})

// 水量每日消耗差值
const getWaterDelta = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_water || 0)
  const prev = parseFloat(logs[i + 1]?.remaining_water || curr)
  const d = curr - prev
  return d >= 0 ? `+${Math.abs(d).toFixed(2)}` : `-${Math.abs(d).toFixed(2)}`
}
const getWaterDeltaColor = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_water || 0)
  const prev = parseFloat(logs[i + 1]?.remaining_water || curr)
  return curr < prev ? 'error' : 'success'
}
const getWaterTrendColor = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_water || 0)
  if (i === 0) return curr === 0 ? 'warning' : 'success'
  const prev = parseFloat(logs[i + 1]?.remaining_water || curr)
  return curr <= prev ? 'error' : 'success'
}
const getWaterTrendIcon = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_water || 0)
  if (i === 0) return curr === 0 ? 'mdi-alert-circle' : 'mdi-check-circle'
  const prev = parseFloat(logs[i + 1]?.remaining_water || curr)
  return curr <= prev ? 'mdi-trending-down' : 'mdi-trending-up'
}

onMounted(async () => {
  try {
    const profileRes = await userAPI.getProfile()
    const profile = profileRes.data.data || {}
    dormRoom.value = profile.dorm_room || ''
    waterDormRoom.value = profile.water_dorm_room || ''
    hasStudentId.value = !!profile.student_id
  } catch {}

  if (dormRoom.value) {
    await loadHistory()
  }
})

onUnmounted(() => {
  clearInterval(refreshTimer)
  clearInterval(countdownTimer)
})
</script>

<style scoped>
.status-card {
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
  border: 1px solid rgba(0,0,0,0.08);
}
.card-warning {
  border-color: rgba(var(--v-theme-warning), 0.4) !important;
  background: rgba(var(--v-theme-warning-container), 0.2) !important;
}
.card-ok {
  border-color: rgba(var(--v-theme-success), 0.2) !important;
  background: rgba(var(--v-theme-success-container), 0.1) !important;
}
</style>
