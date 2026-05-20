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

          <div class="text-caption text-medium-emphasis">
            <v-icon size="12" class="mr-1">mdi-clock-outline</v-icon>
            <span v-if="lastQueryTime">电量更新：{{ lastQueryTime }}</span>
            <span v-else>首次加载中...</span>
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
            <div class="text-overline opacity-60">历史已用水量</div>
              <div class="text-h4 font-weight-bold">
                {{ totalWaterConsumed !== null ? Math.abs(totalWaterConsumed).toFixed(1) : '—' }}
                <span v-if="totalWaterConsumed !== null" class="text-body-1 font-weight-normal">吨</span>
              </div>
              </div>
            </div>
            <v-chip
              :color="waterCardClass"
              text-color="white"
              size="small"
              variant="tonal"
            >
              {{ waterStatusText }}
            </v-chip>
          </div>

          <div v-if="totalWaterConsumed !== null" class="mb-2">
            <div class="text-caption text-medium-emphasis">
              共 {{ historyLogs.filter(l => l.remaining_water).length }} 条记录
            </div>
          </div>
          <div v-else class="mb-2 text-caption text-medium-emphasis">
            暂无水量数据
          </div>

          <div class="text-caption text-medium-emphasis mb-3">
            <v-icon size="12" class="mr-1">mdi-home</v-icon>
            {{ formattedDorm || '未绑定宿舍' }}
          </div>

          <div class="text-caption text-medium-emphasis">
            <v-icon size="12" class="mr-1">mdi-clock-outline</v-icon>
            <span v-if="lastWaterQueryTime">水量更新：{{ lastWaterQueryTime }}</span>
            <span v-else>首次加载中...</span>
          </div>
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
          水电日消耗（近 {{ Math.max(0, historyLogs.length - 1) }} 天）
        </div>
        <div class="d-flex gap-2">
          <v-chip
            :color="powerCardClass === 'card-warning' ? 'warning' : 'primary'"
            text-color="white"
            size="small"
            variant="tonal"
          >
            告警阈值：{{ POWER_THRESHOLD }} 度
          </v-chip>
        </div>
      </div>

      <!-- 趋势折线图 -->
      <div v-if="historyLogs.length > 0" style="height: 280px;">
        <Line :data="chartData" :options="hasWaterHistory ? { ...chartOptions, scales: { ...chartOptions.scales, ...waterScale } } : chartOptions" />
      </div>

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
        <div class="text-caption">点击顶栏刷新按钮获取数据</div>
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, inject } from 'vue'
import { userAPI, powerAPI, waterAPI, systemAPI } from '@/api/index.js'
import emitter from '@/utils/eventBus.js'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS, CategoryScale, LinearScale, PointElement,
  LineElement, Title, Tooltip, Legend, Filler
} from 'chart.js'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

const notify = inject('notify')


const dormRoom      = ref('')
const dormLabel     = ref('')     // 后端映射表返回的标准 Label（如 C10-207）
const currentPower  = ref(null)


const waterDormRoom = ref('')

// 宿舍号展示：优先用后端返回的 dorm_label，否则兼容解析 dorm_room
const formattedDorm = computed(() => {
  return dormLabel.value || dormRoom.value || ''
})
const historyLogs    = ref([])
const queryingPower  = ref(false)
const queryingWater  = ref(false)
const loadingHistory = ref(false)
const lastQueryTime = ref(localStorage.getItem('eq_last_query_time') || '')
const lastWaterQueryTime = ref(localStorage.getItem('eq_last_water_query_time') || '')

// 缓存有效期：5 分钟（毫秒）
const CACHE_TTL = 5 * 60 * 1000
const CACHE_KEY_POWER = 'eq_power_cache'
const CACHE_KEY_WATER = 'eq_water_cache'
const CACHE_KEY_HISTORY = 'eq_history_cache'

// 读取缓存（value + timestamp），过期返回 null
const readCache = (key) => {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    const { value, timestamp } = JSON.parse(raw)
    if (Date.now() - timestamp > CACHE_TTL) return null
    return value
  } catch { return null }
}

// 保存缓存
const saveCache = (key, value) => {
  try {
    localStorage.setItem(key, JSON.stringify({ value, timestamp: Date.now() }))
  } catch {}
}

// 系统配置（从后端获取）
const systemConfig = ref(null)

// 电量阈值：优先从系统配置读取，否则 fallback 到 20
const POWER_THRESHOLD = computed(() => systemConfig.value?.alert_threshold || 20)

// 电量状态
const powerCardClass = computed(() => {
  if (currentPower.value === null) return ''
  return currentPower.value < POWER_THRESHOLD.value ? 'card-warning' : 'card-ok'
})
const powerStatusText = computed(() => {
  if (currentPower.value === null) return '未查询'
  return currentPower.value < POWER_THRESHOLD.value ? '⚠️ 电量不足' : '✅ 电量充足'
})
const powerPercent = computed(() => {
  if (currentPower.value === null) return 0
  return Math.min(100, Math.round((currentPower.value / 100) * 100))
})

// 水量状态（基于透支判断，负数=透支）
const waterCardClass = computed(() => {
  if (totalWaterConsumed.value === null) return 'default'
  return totalWaterConsumed.value < 0 ? 'card-warning' : 'card-ok'
})
const waterStatusText = computed(() => {
  if (totalWaterConsumed.value === null) return '暂无数据'
  return totalWaterConsumed.value < 0 ? '⚠️ 透支中' : '✅ 正常'
})

// 计算日消耗（用于 tooltip 和卡片统计）
// 电量：昨天余额 - 今天余额 = 消耗（正=消耗，负=充值）
// 水量：|昨天 - 今天| = 消耗
const calcDelta = (prev, curr) => {
  if (curr == null || isNaN(curr)) return null
  if (prev == null || isNaN(prev)) return null
  return prev - curr // 昨天 - 今天 = 消耗（充值则负）
}

// 图表数据（按时间正序：旧→新）
const chartData = computed(() => {
  const logs = [...historyLogs.value].reverse() // 旧→新
  if (logs.length < 2) {
    return { labels: [], datasets: [] }
  }

  // 跳过第一条（最旧），从第二条开始计算日消耗
  const labels = logs.slice(1).map(l => {
    const d = new Date(l.record_date)
    return `${d.getMonth() + 1}/${d.getDate()}`
  })

  // 日消耗量数组（索引 i 对应 labels[i]，即第 i+1 天的消耗）
  // logs.slice(1) 跳过最旧那天，l = logs[i+1](当天)，logs[i]=昨天
  const elecDeltas = logs.slice(1).map((l, i) => {
    return calcDelta(parseFloat(logs[i].remaining_kwh), parseFloat(l.remaining_kwh))
  })
  const waterDeltas = logs.slice(1).map((l, i) => {
    const prev = parseFloat(logs[i].remaining_water)
    const curr = parseFloat(l.remaining_water)
    const d = calcDelta(isNaN(prev) ? null : prev, isNaN(curr) ? null : curr)
    return (d != null && !isNaN(d)) ? Math.abs(d) : null // 消耗量取绝对值
  })

  const datasets = [
    {
      label: '用电（度）',
      data: elecDeltas,
      borderColor: '#ff9800',
      backgroundColor: 'rgba(255,152,0,0.1)',
      fill: true,
      tension: 0.3,
      yAxisID: 'y',
      pointRadius: 4,
      pointHoverRadius: 6,
    }
  ]

  if (hasWaterHistory.value) {
    datasets.push({
      label: '用水（吨）',
      data: waterDeltas,
      borderColor: '#29b6f6',
      backgroundColor: 'rgba(41,182,246,0.1)',
      fill: true,
      tension: 0.3,
      yAxisID: 'y1',
      pointRadius: 4,
      pointHoverRadius: 6,
    })
  }

  return { labels, datasets }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index', intersect: false },
  plugins: {
    legend: {
      position: 'top',
      labels: { usePointStyle: true, padding: 20 }
    },
    tooltip: {
      callbacks: {
        label: ctx => ` ${ctx.dataset.label}: ${ctx.parsed.y?.toFixed(2) ?? '—'}`
      }
    }
  },
  scales: {
    x: { grid: { display: false } },
    y: {
      type: 'linear',
      display: true,
      position: 'left',
      title: { display: true, text: '消耗（度）' },
      grid: { color: 'rgba(0,0,0,0.05)' },
    },
  }
}

// 水量专用 Y 轴（仅水量有数据时追加）
const waterScale = computed(() => ({
  y1: {
    type: 'linear',
    display: true,
    position: 'right',
    title: { display: true, text: '消耗（吨）' },
    grid: { drawOnChartArea: false },
  }
}))

// 查询电量（仅更新电量，不影响水量）
const queryNow = async () => {
  queryingPower.value = true
  try {
    const res = await powerAPI.current()
    const data = res.data.data
    currentPower.value = parseFloat(data?.remaining_kwh)
    const now = new Date().toLocaleTimeString('zh-CN')
    lastQueryTime.value = now
    // 存入缓存（含时间戳供缓存回显）
    saveCache(CACHE_KEY_POWER, { v: currentPower.value, _time: now })
    localStorage.setItem('eq_last_query_time', now)
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
    const res = await waterAPI.balance()
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

// 加载历史记录（每次调用都从后端拉最新数据，并写入缓存）
const loadHistory = async () => {
  if (!dormRoom.value) return
  loadingHistory.value = true
  try {
    const res = await powerAPI.records(14)
    historyLogs.value = res.data.data || []
    // 写入趋势缓存（含 historyLogs 完整数据，供页面切换时回显）
    saveCache(CACHE_KEY_HISTORY, historyLogs.value)
    if (historyLogs.value.length > 0) {
      if (currentPower.value === null) {
        currentPower.value = parseFloat(historyLogs.value[0].remaining_kwh)
      }
    }
    // 只要宿舍配置了水量，就写入时间戳缓存
    if (waterDormRoom.value) {
      const now = new Date().toLocaleTimeString('zh-CN')
      lastWaterQueryTime.value = now
      saveCache(CACHE_KEY_WATER, { _time: now })
      localStorage.setItem('eq_last_water_query_time', now)
    }
  } catch (e) { console.warn('加载历史记录失败', e) }
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
  if (i === 0) return curr < POWER_THRESHOLD.value ? 'warning' : 'success'
  return curr <= prev ? 'error' : 'success'
}
const getTrendIcon = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_kwh)
  const prev = parseFloat(logs[i + 1]?.remaining_kwh || curr)
  if (i === 0) return curr < POWER_THRESHOLD.value ? 'mdi-alert-circle' : 'mdi-check-circle'
  return curr <= prev ? 'mdi-trending-down' : 'mdi-trending-up'
}

// 水量历史：是否存在水量数据（C13/C14 楼才有）
const hasWaterHistory = computed(() => {
  return historyLogs.value.some(item => item.remaining_water && item.remaining_water.trim() !== '')
})

// 历史累计消耗：remaining_water 是预付费账户余额（正数 = 剩余水量）
// totalWaterConsumed = 最新记录余额（累计已购水量），取绝对值用于显示
const totalWaterConsumed = computed(() => {
  const logs = historyLogs.value
  if (logs.length === 0) return null
  // 取第一条（最新）的 remaining_water
  const latest = logs[0]?.remaining_water
  if (!latest && latest !== 0) return null
  return Math.abs(parseFloat(latest))
})

// 水量每日消耗差值
const getWaterDelta = (logs, i) => {
  const curr = parseFloat(logs[i].remaining_water || 0)
  const prev = parseFloat(logs[i + 1]?.remaining_water || curr)
  // remaining_water 是负值（消耗量），-164 → -163（消耗减少）→ d = -1 → 取绝对值显示
  const d = Math.abs(prev - curr)
  return `+${d.toFixed(2)}`
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

// 防止快速切换时多次触发 onMounted
const isMounted = ref(false)

onMounted(async () => {
  if (isMounted.value) return
  isMounted.value = true

  // 并行加载系统配置、用户信息
  await Promise.all([
    (async () => {
      try {
        const cfgRes = await systemAPI.getConfig()
        systemConfig.value = cfgRes.data.data
      } catch (e) { console.warn('加载系统配置失败', e) }
    })(),
    (async () => {
      try {
        const profileRes = await userAPI.getProfile()
        const profile = profileRes.data.data || {}
        dormRoom.value = profile.dorm_room || ''
        dormLabel.value = profile.dorm_label || ''
        waterDormRoom.value = profile.water_dorm_room || ''
      } catch (e) { console.warn('加载用户信息失败', e) }
    })(),
  ])

  // 优先从缓存恢复（页面切换时直接展示缓存数据，不等待 API）
  const cachedPower = readCache(CACHE_KEY_POWER)
  if (cachedPower !== null) {
    currentPower.value = cachedPower.v
    lastQueryTime.value = cachedPower._time || ''
  }
  const cachedWater = readCache(CACHE_KEY_WATER)
  if (cachedWater !== null) {
    lastWaterQueryTime.value = cachedWater._time || ''
  }
  const cachedHistory = readCache(CACHE_KEY_HISTORY)
  if (cachedHistory !== null && cachedHistory.length > 0) {
    historyLogs.value = cachedHistory
    // 电量也从历史缓存兜底恢复（避免历史有数据但电量缓存丢失的情况）
    if (currentPower.value === null) {
      currentPower.value = parseFloat(cachedHistory[0].remaining_kwh)
    }
  }

  if (dormRoom.value) {
    if (!cachedPower) await queryNow()       // 无电量缓存才查
    if (!cachedWater && waterDormRoom.value) await queryWater()  // 无水量缓存才查
    if (!cachedHistory) await loadHistory()  // 无趋势缓存才查
  }

  // 自动刷新定时器：每 5 分钟检查缓存是否过期，过期则自动刷新
  let refreshTimer = setInterval(() => {
    if (!dormRoom.value) return
    const cp = readCache(CACHE_KEY_POWER)
    const cw = readCache(CACHE_KEY_WATER)
    if (!cp) queryNow()
    if (!cw && waterDormRoom.value) queryWater()
  }, 5 * 60 * 1000)

  // 监听顶栏刷新按钮
  const onManualRefresh = () => {
    clearInterval(refreshTimer)    // 重置自动定时器，避免与手动刷新重叠
    queryNow()
    if (waterDormRoom.value) queryWater()
    // 重新启动 5 分钟自动刷新
    refreshTimer = setInterval(() => {
      if (!dormRoom.value) return
      const cp = readCache(CACHE_KEY_POWER)
      const cw = readCache(CACHE_KEY_WATER)
      if (!cp) queryNow()
      if (!cw && waterDormRoom.value) queryWater()
    }, 5 * 60 * 1000)
  }
  emitter.on('refresh', onManualRefresh)

  onUnmounted(() => {
    clearInterval(refreshTimer)
    emitter.off('refresh', onManualRefresh)
  })
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
