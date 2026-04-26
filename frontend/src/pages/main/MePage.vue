<template>
  <div>
    <!-- ====== 顶部用户卡片 ====== -->
    <v-card class="mb-4 pa-4 user-card" elevation="0">
      <div class="d-flex align-center">
        <v-avatar size="56" color="primary" class="mr-4">
          <span class="text-h5 font-weight-bold text-white">{{ userInitial }}</span>
        </v-avatar>
        <div class="flex-grow-1">
          <div class="text-subtitle-1 font-weight-bold">{{ displayName }}</div>
          <div class="text-caption text-medium-emphasis">
            {{ profile.student_id ? `学号：${profile.student_id}` : '未绑定学号' }}
          </div>
        </div>
        <v-icon
          size="20"
          :color="profile.dorm_room ? 'success' : 'grey'"
          class="mr-2"
        >
          {{ profile.dorm_room ? 'mdi-lock-open-check' : 'mdi-lock-outline' }}
        </v-icon>
        <v-chip
          :color="profile.dorm_room ? 'success' : 'grey'"
          text-color="white"
          size="small"
          variant="tonal"
        >
          {{ profile.dorm_room ? '已绑定宿舍' : '未绑定宿舍' }}
        </v-chip>
      </div>
    </v-card>

    <!-- ====== 标签页切换 ====== -->
    <v-card elevation="0">
      <v-tabs v-model="activeTab" color="primary" align-tabs="start">
        <v-tab value="profile" class="text-none">
          <v-icon start size="18">mdi-account-edit</v-icon>
          个人信息
        </v-tab>
        <v-tab value="channels" class="text-none">
          <v-icon start size="18">mdi-bell-cog</v-icon>
          通知设置
        </v-tab>
        <v-tab value="security" class="text-none">
          <v-icon start size="18">mdi-shield-account</v-icon>
          账号安全
        </v-tab>
      </v-tabs>

      <v-divider />

      <v-window v-model="activeTab" class="pa-4">
        <!-- ====== 个人信息 Tab ====== -->
        <v-window-item value="profile">
          <!-- 基本信息 -->
          <v-form ref="formRef" @submit.prevent="saveProfile">
            <v-text-field
              v-model="form.name"
              label="姓名"
              prepend-inner-icon="mdi-account"
              placeholder="方便通知时称呼您"
              class="mb-3"
            />

            <v-text-field
              v-model="form.student_id"
              label="学号"
              prepend-inner-icon="mdi-identifier"
              placeholder="选填，用于绑定一卡通"
              class="mb-3"
            />

            <v-text-field
              v-model="form.class"
              label="班级"
              prepend-inner-icon="mdi-school"
              placeholder="如：高分子2301"
              class="mb-3"
            />

            <v-divider class="my-4" />
            <div class="d-flex align-center mb-3">
              <v-icon class="mr-1 text-primary" size="16">mdi-home-city</v-icon>
              <span class="text-subtitle-2 font-weight-bold">宿舍信息</span>
            </div>

            <!-- 电费宿舍：三级联动下拉（有缓存时）/ 手动输入（无缓存时） -->
            <!-- 降级提示：数据库为空时提示服务正在初始化 -->
            <v-alert
              v-if="dormCacheLoaded && dormOptions.buildings.length === 0"
              type="info"
              variant="tonal"
              class="mb-3"
              density="compact"
            >
              <template #prepend>
                <v-icon size="18">mdi-clock-outline</v-icon>
              </template>
              <div class="text-body-2">
                <strong>服务初始化中</strong>——房间列表正在从官网同步，通常需要几分钟。<br/>
                请稍候后点击「刷新选项」，或手动填写宿舍号（如 <code>140328</code>）。
              </div>
            </v-alert>

            <template v-if="dormCacheLoaded && dormOptions.buildings.length > 0">
              <v-row dense class="mb-2">
                <v-col cols="12" sm="4">
                  <v-select
                    v-model="selectedElectric.building"
                    label="宿舍楼"
                    prepend-inner-icon="mdi-office-building"
                    :items="electricBuildingItems"
                    placeholder="选择楼栋"
                    clearable
                    density="comfortable"
                    @update:model-value="onElectricBuildingChange"
                  />
                </v-col>
                <v-col cols="12" sm="4">
                  <v-select
                    v-model="selectedElectric.floor"
                    label="楼层"
                    prepend-inner-icon="mdi-stairs"
                    :items="electricFloorItems"
                    :disabled="!selectedElectric.building"
                    placeholder="选择楼层"
                    clearable
                    density="comfortable"
                    @update:model-value="onElectricFloorChange"
                  />
                </v-col>
                <v-col cols="12" sm="4">
                  <v-select
                    v-model="selectedElectric.roomFormValue"
                    label="房间"
                    prepend-inner-icon="mdi-door"
                    :items="electricRoomItems"
                    :disabled="!selectedElectric.floor"
                    placeholder="选择房间"
                    clearable
                    density="comfortable"
                    @update:model-value="onElectricRoomChange"
                  />
                </v-col>
              </v-row>

              <!-- 已选宿舍展示 -->
              <div v-if="selectedElectric.label" class="d-flex align-center mb-2">
                <v-icon size="16" color="success" class="mr-1">mdi-check-circle</v-icon>
                <span class="text-body-2 text-success">已选：{{ selectedElectric.label }}</span>
                <v-btn
                  size="x-small"
                  variant="text"
                  color="primary"
                  class="ml-2"
                  :loading="validatingDorm"
                  @click="validateDormRoom"
                >
                  校验
                </v-btn>
              </div>
            </template>

            <!-- 无缓存时：手动输入 -->
            <template v-else>
              <v-row dense class="mb-2">
                <v-col cols="12" sm="4">
                  <v-select
                    v-model="form.building"
                    label="宿舍楼"
                    prepend-inner-icon="mdi-office-building"
                    :items="buildingOptions"
                    placeholder="选择楼栋"
                    clearable
                    density="comfortable"
                  />
                </v-col>
                <v-col cols="12" sm="4">
                  <v-text-field
                    v-model="form.floor"
                    label="楼层"
                    prepend-inner-icon="mdi-stairs"
                    placeholder="如：3"
                    clearable
                    density="comfortable"
                  />
                </v-col>
                <v-col cols="12" sm="4">
                  <v-text-field
                    v-model="form.room"
                    label="房间号"
                    prepend-inner-icon="mdi-door"
                    placeholder="如：1301"
                    clearable
                    density="comfortable"
                  />
                </v-col>
              </v-row>

              <v-text-field
                :model-value="fullDormRoom"
                label="完整宿舍号"
                prepend-inner-icon="mdi-text-box"
                readonly
                variant="outlined"
                density="comfortable"
                hint="上方选择或手动输入完整格式"
                persistent-hint
                class="mb-2"
              >
                <template #append-inner>
                  <v-btn
                    icon="mdi-refresh"
                    variant="text"
                    size="small"
                    :loading="validatingDorm"
                    title="校验宿舍"
                    @click="validateDormRoom"
                  />
                </template>
              </v-text-field>
            </template>

            <!-- 校验结果 -->
            <v-expand-transition>
              <v-alert
                v-if="dormValidateResult"
                :type="dormValid ? 'success' : 'error'"
                variant="tonal"
                density="compact"
                rounded="lg"
                class="mb-3"
                :icon="dormValid ? 'mdi-check-circle' : 'mdi-alert-circle'"
              >
                {{ dormValidateResult }}
              </v-alert>
            </v-expand-transition>

            <v-btn
              type="submit"
              color="primary"
              size="large"
              block
              :loading="savingProfile"
              :disabled="!hasChanges"
            >
              <v-icon start>mdi-content-save</v-icon>
              保存信息
            </v-btn>
          </v-form>
        </v-window-item>

        <!-- ====== 通知设置 Tab ====== -->
        <v-window-item value="channels">
          <!-- 企业微信卡片 -->
          <v-card class="mb-4 pa-4" elevation="0">
            <div class="d-flex align-center mb-3">
              <v-avatar color="green-lighten-4" size="40" class="mr-3">
                <v-icon color="green-darken-2" size="22">mdi-wechat</v-icon>
              </v-avatar>
              <div class="flex-grow-1">
                <div class="text-subtitle-2 font-weight-bold">企业微信机器人</div>
                <div class="text-caption text-medium-emphasis">通过群机器人 Webhook 接收通知</div>
              </div>
              <v-chip
                :color="channelForm.wechat_webhook ? 'success' : 'grey'"
                text-color="white"
                size="small"
                variant="tonal"
              >
                {{ channelForm.wechat_webhook ? '已配置' : '未配置' }}
              </v-chip>
            </div>

            <v-text-field
              v-model="channelForm.wechat_webhook"
              label="Webhook URL"
              prepend-inner-icon="mdi-link-variant"
              placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..."
              clearable
              class="mb-2"
              :rules="[rules.url]"
              density="comfortable"
            />

            <v-alert
              type="info"
              variant="tonal"
              density="compact"
              rounded="lg"
              class="mb-0"
              icon="mdi-information-outline"
            >
              <div class="text-caption">添加机器人：企业微信群 → 群设置 → 群机器人 → 添加机器人</div>
            </v-alert>
          </v-card>

          <!-- 邮件通知卡片 -->
          <v-card class="mb-4 pa-4" elevation="0">
            <div class="d-flex align-center mb-3">
              <v-avatar color="blue-lighten-4" size="40" class="mr-3">
                <v-icon color="blue-darken-2" size="22">mdi-email</v-icon>
              </v-avatar>
              <div class="flex-grow-1">
                <div class="text-subtitle-2 font-weight-bold">邮件通知</div>
                <div class="text-caption text-medium-emphasis">接收电量告警和每周用电报告</div>
              </div>
              <v-chip
                :color="channelForm.email ? 'success' : 'grey'"
                text-color="white"
                size="small"
                variant="tonal"
              >
                {{ channelForm.email ? '已配置' : '未配置' }}
              </v-chip>
            </div>

            <v-text-field
              v-model="channelForm.email"
              label="接收邮箱"
              prepend-inner-icon="mdi-at"
              placeholder="your.name@email.com"
              type="email"
              clearable
              :rules="[rules.email]"
              density="comfortable"
            />
          </v-card>

          <!-- 保存 & 测试按钮 -->
          <v-card class="pa-4" elevation="0">
            <v-expand-transition>
              <v-alert
                v-if="testResult !== null"
                :type="testResult ? 'success' : 'error'"
                variant="tonal"
                rounded="lg"
                class="mb-4"
                :icon="testResult ? 'mdi-check-circle' : 'mdi-alert-circle'"
              >
                {{ testResult ? '测试通知发送成功！' : '测试通知发送失败，请检查配置' }}
              </v-alert>
            </v-expand-transition>

            <div class="d-flex align-center">
              <v-btn
                color="primary"
                size="large"
                class="flex-grow-1 mr-3"
                :loading="savingChannel"
                @click="saveChannels(false)"
              >
                <v-icon start>mdi-content-save</v-icon>
                保存配置
              </v-btn>
              <v-btn
                variant="outlined"
                color="primary"
                size="large"
                :loading="testing"
                :disabled="!channelForm.wechat_webhook && !channelForm.email"
                @click="saveChannels(true)"
              >
                <v-icon start>mdi-send</v-icon>
                测试
              </v-btn>
            </div>

            <v-divider class="my-4" />

            <!-- 触发场景说明 -->
            <v-card variant="outlined" class="pa-4" elevation="0">
              <div class="d-flex align-center mb-3">
                <v-icon color="primary" size="18" class="mr-2">mdi-bell-ring</v-icon>
                <div class="text-subtitle-2 font-weight-bold">通知触发场景</div>
              </div>
              <div class="scene-item">
                <v-icon size="16" color="warning" class="mr-2 flex-shrink-0">mdi-lightning-bolt</v-icon>
                <div class="text-body-2">电量低于 {{ systemConfig?.alert_threshold || 20 }} 度时自动告警</div>
              </div>
              <div class="scene-item mt-3">
                <v-icon size="16" color="info" class="mr-2 flex-shrink-0">mdi-calendar-week</v-icon>
                <div class="text-body-2">每周{{ weekdayName(systemConfig?.weekly_report_weekday || 1) }}早 {{ systemConfig?.weekly_report_hour || 8 }}:00 发送用电周报</div>
              </div>
              <div class="scene-item mt-3">
                <v-icon size="16" color="success" class="mr-2 flex-shrink-0">mdi-flash</v-icon>
                <div class="text-body-2">保存配置后可立即发送测试通知</div>
              </div>
            </v-card>

            <!-- 隐私说明 -->
            <v-card variant="outlined" class="pa-4 mt-4" elevation="0">
              <div class="d-flex align-center mb-2">
                <v-icon color="medium-emphasis" size="18" class="mr-2">mdi-shield-check</v-icon>
                <div class="text-subtitle-2 font-weight-bold">隐私说明</div>
              </div>
              <div class="text-caption text-medium-emphasis">
                您的通知渠道凭证仅用于向您本人发送通知，绝不会用于其他用途。
              </div>
            </v-card>
          </v-card>
        </v-window-item>

        <!-- ====== 账号安全 Tab ====== -->
        <v-window-item value="security">
          <!-- 账号信息卡片 -->
          <v-card class="mb-4 pa-4" elevation="0">
            <div class="d-flex align-center mb-3">
              <v-icon class="mr-2 text-primary" size="20">mdi-account-circle</v-icon>
              <span class="text-subtitle-2 font-weight-bold">账号信息</span>
            </div>
            <div class="d-flex align-center mb-3">
              <div class="text-body-2 text-medium-emphasis" style="width:80px">账号</div>
              <div class="text-body-2 font-weight-medium">{{ profile.username || '—' }}</div>
            </div>
            <div class="d-flex align-center mb-3">
              <div class="text-body-2 text-medium-emphasis" style="width:80px">注册时间</div>
              <div class="text-body-2">{{ formatDate(profile.created_at) }}</div>
            </div>
            <div class="d-flex align-center">
              <div class="text-body-2 text-medium-emphasis" style="width:80px">用户 ID</div>
              <div class="text-body-2 font-weight-medium">{{ profile.id }}</div>
            </div>
          </v-card>

          <!-- 修改密码卡片 -->
          <v-card class="mb-4 pa-4" elevation="0">
            <div class="d-flex align-center mb-3">
              <v-icon class="mr-2 text-primary" size="20">mdi-lock-reset</v-icon>
              <span class="text-subtitle-2 font-weight-bold">修改密码</span>
            </div>
            <v-form ref="pwdFormRef" @submit.prevent="doChangePwd">
              <v-text-field
                v-model="pwdForm.old_password"
                label="当前密码"
                prepend-inner-icon="mdi-lock-outline"
                type="password"
                autocomplete="current-password"
                placeholder="请输入当前密码"
                :rules="[v => !!v || '请输入当前密码']"
                class="mb-3"
                density="comfortable"
              />
              <v-text-field
                v-model="pwdForm.new_password"
                label="新密码"
                prepend-inner-icon="mdi-lock-plus"
                type="password"
                autocomplete="new-password"
                placeholder="至少 8 位"
                :rules="pwdRules"
                class="mb-3"
                density="comfortable"
              />
              <v-text-field
                v-model="pwdForm.confirm_password"
                label="确认新密码"
                prepend-inner-icon="mdi-lock-check"
                type="password"
                autocomplete="new-password"
                placeholder="再次输入新密码"
                :rules="[v => v === pwdForm.new_password || '两次密码不一致']"
                class="mb-3"
                density="comfortable"
              />
              <v-alert
                v-if="pwdSuccess"
                type="success"
                variant="tonal"
                density="compact"
                rounded="lg"
                class="mb-3"
              >
                密码修改成功！
              </v-alert>
              <v-btn
                type="submit"
                color="primary"
                :loading="changingPwd"
                :disabled="!hasPwdChanges"
                block
              >
                <v-icon start>mdi-content-save</v-icon>
                修改密码
              </v-btn>
            </v-form>
          </v-card>

          <!-- 两步验证卡片 -->
          <v-card class="mb-4 pa-4" elevation="0">
            <div class="d-flex align-center mb-3">
              <v-icon class="mr-2 text-primary" size="20">mdi-two-factor-authentication</v-icon>
              <span class="text-subtitle-2 font-weight-bold">两步验证</span>
              <v-spacer />
              <v-chip
                :color="profile.totp_enabled ? 'success' : 'grey'"
                text-color="white"
                size="small"
                variant="tonal"
              >
                {{ profile.totp_enabled ? '已启用' : '未启用' }}
              </v-chip>
            </div>

            <!-- 未启用状态 -->
            <template v-if="!profile.totp_enabled">
              <div class="text-body-2 text-medium-emphasis mb-3">
                启用后，登录时需额外输入 Authenticator 应用（如 Google Authenticator）生成的动态验证码，大幅提升账号安全。
              </div>
              <v-btn
                color="primary"
                :loading="totpSetupLoading"
                @click="openSetupTotp"
              >
                <v-icon start>mdi-qrcode</v-icon>
                启用两步验证
              </v-btn>
            </template>

            <!-- 已启用状态 -->
            <template v-else>
              <div class="text-body-2 text-medium-emphasis mb-3">
                两步验证已启用，每次登录时需要输入 Authenticator 生成的验证码。
              </div>
              <v-btn
                color="error"
                variant="outlined"
                :loading="disablingTotp"
                @click="openDisableTotp"
              >
                <v-icon start>mdi-close-circle</v-icon>
                关闭两步验证
              </v-btn>
            </template>
          </v-card>
        </v-window-item>
      </v-window>
    </v-card>
  </div>

  <!-- ====== TOTP 设置对话框（扫码启用） ====== -->
  <v-dialog v-model="totpSetupDialog" max-width="420" persistent>
    <v-card class="pa-5" elevation="0">
      <div class="text-subtitle-1 font-weight-bold mb-1">
        <v-icon color="primary" class="mr-1">mdi-qrcode</v-icon>
        设置两步验证
      </div>
      <div class="text-body-2 text-medium-emphasis mb-4">
        请使用 Authenticator 应用（如 Google Authenticator）扫描下方二维码，
        然后输入屏幕上显示的 6 位验证码完成启用。
      </div>

      <!-- 二维码 -->
      <div v-if="totpQrCode" class="text-center mb-4">
        <img :src="totpQrCode" alt="TOTP QR Code" style="width:200px;height:200px;border-radius:8px;border:1px solid rgba(var(--v-theme-outline),0.2)" />
      </div>
      <div v-else-if="totpSetupError" class="text-center mb-4">
        <v-alert type="error" variant="tonal" density="compact">{{ totpSetupError }}</v-alert>
      </div>
      <div v-else class="text-center mb-4">
        <v-progress-circular indeterminate color="primary" />
        <div class="text-caption text-medium-emphasis mt-2">生成中...</div>
      </div>

      <!-- 密钥（备用，手动输入） -->
      <div v-if="totpSecret" class="mb-4">
        <div class="text-caption text-medium-emphasis mb-1">手动密钥（若无法扫码）</div>
        <code style="word-break:break-all;background:rgba(var(--v-theme-surface-variant));padding:4px 8px;border-radius:4px;display:block">{{ totpSecret }}</code>
      </div>

      <!-- 验证码输入 -->
      <v-text-field
        v-model="totpVerifyCode"
        label="输入 6 位验证码"
        placeholder="000000"
        maxlength="6"
        :rules="[v => /^\d{6}$/.test(v) || '请输入 6 位数字验证码']"
        class="mb-3"
        density="comfortable"
      />

      <v-alert v-if="totpVerifyError" type="error" variant="tonal" density="compact" class="mb-3">
        {{ totpVerifyError }}
      </v-alert>

      <div class="d-flex gap-2">
        <v-btn variant="text" class="flex-grow-1" @click="closeTotpSetup">取消</v-btn>
        <v-btn
          color="primary"
          class="flex-grow-1"
          :loading="verifyingTotp"
          :disabled="!totpVerifyCode || totpVerifyCode.length !== 6"
          @click="doEnableTotp"
        >
          确认启用
        </v-btn>
      </div>
    </v-card>
  </v-dialog>

  <!-- ====== TOTP 关闭确认对话框 ====== -->
  <v-dialog v-model="totpDisableDialog" max-width="380">
    <v-card class="pa-5" elevation="0">
      <div class="text-subtitle-1 font-weight-bold mb-3">
        <v-icon color="error" class="mr-1">mdi-alert</v-icon>
        关闭两步验证
      </div>
      <div class="text-body-2 mb-3">
        关闭两步验证会降低账号安全等级。请输入当前登录密码以确认操作。
      </div>
      <v-text-field
        v-model="totpDisablePwd"
        label="当前密码"
        prepend-inner-icon="mdi-lock-outline"
        type="password"
        autocomplete="current-password"
        :rules="[v => !!v || '请输入密码']"
        class="mb-3"
        density="comfortable"
      />
      <v-alert v-if="totpDisableError" type="error" variant="tonal" density="compact" class="mb-3">
        {{ totpDisableError }}
      </v-alert>
      <div class="d-flex gap-2">
        <v-btn variant="text" class="flex-grow-1" @click="totpDisableDialog = false">取消</v-btn>
        <v-btn
          color="error"
          class="flex-grow-1"
          :loading="disablingTotp"
          @click="doDisableTotp"
        >
          确认关闭
        </v-btn>
      </div>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch, onMounted, inject } from 'vue'
import { userAPI, dormAPI, systemAPI } from '@/api/index.js'
import QRCode from 'qrcode'

const notify = inject('notify')

// ---- Tab 控制 ----
const activeTab = ref('profile')

// ---- 系统配置（从后端获取）----
const systemConfig = ref(null)

// ---- 用户信息 ----
const displayName = computed(() => {
  const stored = JSON.parse(localStorage.getItem('eq_user') || '{}')
  return stored?.name || stored?.student_id || '用户'
})
const userInitial = computed(() => (displayName.value || 'U')[0].toUpperCase())

// ---- 个人信息 ----
const profile = reactive({})
const form    = reactive({
  name: '', student_id: '', class: '',
  building: '', floor: '', room: '', dorm_room: '',
})
const savingProfile = ref(false)
const validatingDorm        = ref(false)
const dormValid            = ref(false)
const dormValidateResult   = ref('')
const formRef = ref(null)

// ---- 下拉选项缓存 ----
const dormOptions = reactive({ buildings: [], floors: [], rooms: [] })
const dormCacheLoaded = ref(false)
const loadingDormCache = ref(false)

// 电费宿舍选择状态
const selectedElectric = reactive({
  building: '',
  floor: '',
  roomFormValue: '',  // 表单实际值（drceng）
  label: '',          // 显示标签（如 "C11-1101-132水表"）
})

// ---- 反向同步：profile.dorm_room → selectedElectric ----
// FormValue 现为完整宿舍号（如 110132水表），直接精确匹配即可回显
const reverseSyncFromProfile = () => {
  if (!dormCacheLoaded.value) return

  // 同步表单字段
  form.name = profile.name || ''
  form.student_id = profile.student_id || ''
  form.class = profile.class || ''

  // 电费宿舍：FormValue = 完整宿舍号，直接精确匹配
  if (profile.dorm_room) {
    const matched = dormOptions.rooms.find(r => r.form_value === profile.dorm_room)
    if (matched) {
      selectedElectric.building = matched.building
      selectedElectric.floor = matched.floor
      selectedElectric.roomFormValue = matched.form_value
      selectedElectric.label = matched.label
      form.dorm_room = matched.form_value
    }
  }
}

// 监听缓存加载完成 → 触发反向同步
watch(dormCacheLoaded, (loaded) => {
  if (loaded) reverseSyncFromProfile()
})

// 加载下拉选项
const loadDormOptions = async () => {
  loadingDormCache.value = true
  try {
    const res = await dormAPI.getOptions()
    const data = res.data.data
    dormOptions.buildings = data.buildings || []
    dormOptions.floors = data.floors || []
    dormOptions.rooms = data.rooms || []
    dormCacheLoaded.value = true

    if (dormOptions.buildings.length === 0) {
      notify('房间列表为空，请稍后刷新页面重试', 'info')
    } else {
      reverseSyncFromProfile()
    }
  } catch (err) {
    notify('加载下拉选项失败', 'error')
  } finally {
    loadingDormCache.value = false
  }
}

// ---- 电费宿舍三级联动 ----
// FormValue = 完整宿舍号（如 110132水表），Label = 官网同款显示名（如 C11-132水表）
const electricBuildingItems = computed(() =>
  dormOptions.buildings.map(b => ({ title: b.label, value: b.form_value }))
)
// 格式化楼层 ablou → 友好显示名（如 "1002" → "2层"，"10" → "10层"）
// ablou 格式：building(2)+floor(2)，取后2位并去掉前导零
const formatFloor = (building, ablou) => {
  if (!ablou || ablou.length < 2) return ablou
  const floorPart = ablou.slice(2) // 去掉 building 前缀，取后2位楼层
  const num = parseInt(floorPart, 10)
  return isNaN(num) ? floorPart : num + '层'
}

// 去掉标签中的楼栋前缀，显示纯房间号（如 "C10-207" → "207"，"C10-207水表" → "207水表"）
const stripBuildingPrefix = (label) => {
  if (!label) return ''
  const dashIdx = label.indexOf('-')
  return dashIdx >= 0 ? label.slice(dashIdx + 1) : label
}

const electricFloorItems = computed(() =>
  dormOptions.floors
    .filter(f => f.building === selectedElectric.building)
    .map(f => ({ title: formatFloor(f.building, f.label), value: f.form_value }))
)
const electricRoomItems = computed(() => {
  // 杂数据关键词
  const noiseKeywords = ['总表', '公共', '台盆', '洗衣间', '茶水间', '备用', '公用']
  const isNoise = (label) => noiseKeywords.some(kw => label && label.includes(kw))

  return dormOptions.rooms
    .filter(r => {
      // 过滤条件：
      // 1. 同楼同层
      // 2. 不是杂数据（总表/公共/台盆/洗衣间/茶水间/备用/公用）
      // 3. 不是水表房间（由水费宿舍单独选择）
      const isNoiseRoom = isNoise(r.label) || isNoise(r.form_value)
      const isWaterRoom = r.label && r.label.includes('水表')
      return r.building === selectedElectric.building && r.floor === selectedElectric.floor && !isNoiseRoom && !isWaterRoom
    })
    .map(r => ({ title: stripBuildingPrefix(r.label), value: r.form_value }))
})

const onElectricBuildingChange = () => {
  selectedElectric.floor = ''
  selectedElectric.roomFormValue = ''
  selectedElectric.label = ''
}

const onElectricFloorChange = () => {
  selectedElectric.roomFormValue = ''
  selectedElectric.label = ''
}

// 选中房间：前端显示 label，后端直接存 form_value（完整宿舍号，ParseDorm 可解析）
const onElectricRoomChange = () => {
  const room = dormOptions.rooms.find(r =>
    r.building === selectedElectric.building &&
    r.floor === selectedElectric.floor &&
    r.form_value === selectedElectric.roomFormValue
  )
  if (room) {
    selectedElectric.label = room.label
    form.dorm_room = selectedElectric.roomFormValue // 完整宿舍号
  } else {
    selectedElectric.label = ''
  }
}



const fullDormRoom = computed(() => {
  // 缓存加载后 form.dorm_room 已存完整宿舍号（如 110132水表），直接返回
  if (dormCacheLoaded.value) return form.dorm_room || ''
  // 无缓存时：手动输入模式，拼装旧逻辑
  if (form.dorm_room) return form.dorm_room
  if (form.building && form.room) {
    const bldNum = form.building.replace(/^C/i, '')
    if (form.floor) return bldNum + '0' + form.floor + form.room
    return bldNum + '-' + form.room
  }
  return ''
})

const hasChanges = computed(() => {
  return form.name            !== (profile.name             || '')
      || form.student_id      !== (profile.student_id      || '')
      || form.class           !== (profile.class           || '')
      || form.dorm_room       !== (profile.dorm_room      || '')
      || selectedElectric.roomFormValue !== (profile.dorm_room || '')
})

const validateDormRoom = async () => {
  const dorm = fullDormRoom.value
  if (!dorm) {
    dormValidateResult.value = '请先选择或填写电费宿舍号'
    dormValid.value = false
    return
  }
  validatingDorm.value = true
  dormValidateResult.value = ''
  try {
    const res = await userAPI.validateDorm({ dorm_room: dorm })
    const data = res.data.data
    dormValid.value = data.valid
    dormValidateResult.value = data.message
    if (!data.valid) notify(data.message, 'error')
  } catch (err) {
    dormValid.value = false
    dormValidateResult.value = err.response?.data?.msg || '校验失败'
  } finally {
    validatingDorm.value = false
  }
}

const saveProfile = async () => {
  savingProfile.value = true
  try {
    const payload = {
      name:             form.name,
      student_id:        form.student_id || null,
      class:            form.class,
      dorm_room:        fullDormRoom.value,
    }
    const res = await userAPI.updateProfile(payload)
    Object.assign(profile, res.data.data)
    updateStoredUser(res.data.data)
    notify('个人信息已保存')
  } catch (err) {
    notify(err.response?.data?.msg || '保存失败', 'error')
  } finally {
    savingProfile.value = false
  }
}

const saveChannels = async (forceTest = false) => {
  const hasAnyChannel = channelForm.wechat_webhook || channelForm.email
  if (!hasAnyChannel) {
    notify('请至少填写一种通知渠道', 'warning')
    return
  }
  savingChannel.value = true
  testResult.value = null
  try {
    await userAPI.updateChannel({
      wechat_webhook: String(channelForm.wechat_webhook || ''),
      email: String(channelForm.email || ''),
      test_channel: !!forceTest,
    })
    notify(forceTest ? '配置已保存，正在发送测试通知...' : '配置已保存')
  } catch (err) {
    notify(err.response?.data?.msg || '保存失败', 'error')
  } finally {
    savingChannel.value = false
  }
}

const updateStoredUser = (data) => {
  const stored = JSON.parse(localStorage.getItem('eq_user') || '{}')
  Object.assign(stored, data)
  localStorage.setItem('eq_user', JSON.stringify(stored))
}

const formatDate = (iso) => {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('zh-CN')
}

// ---- 通知渠道 ----
const channelForm = reactive({ wechat_webhook: '', email: '' })
const savingChannel = ref(false)
const testing       = ref(false)
const testResult     = ref(null)

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

// ---- 楼栋选项 ----
const buildingOptions = [
  { title: '01 楼', value: '01' }, { title: '02 楼', value: '02' },
  { title: '03 楼', value: '03' }, { title: '04 楼', value: '04' },
  { title: '05 楼', value: '05' }, { title: '06 楼', value: '06' },
  { title: '07 楼', value: '07' }, { title: '08 楼', value: '08' },
  { title: '09 楼', value: '09' }, { title: '10 楼', value: '10' },
  { title: '11 楼', value: '11' }, { title: '12 楼', value: '12' },
  { title: '13 楼', value: '13' }, { title: '14 楼', value: '14' },
]

// 星期数字转中文
const WEEKDAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const weekdayName = (n) => WEEKDAY_NAMES[n] || '周一'

// 防止快速切换时多次触发 onMounted
let isMounted = false

onMounted(async () => {
  if (isMounted) return
  isMounted = true

  // 加载系统配置
  try {
    const cfgRes = await systemAPI.getConfig()
    systemConfig.value = cfgRes.data.data
  } catch {}

  // 加载个人信息
  try {
    const res = await userAPI.getProfile()
    Object.assign(profile, res.data.data)
    form.name             = profile.name             || ''
    form.student_id       = profile.student_id       || ''
    form.class            = profile.class           || ''
    form.dorm_room        = profile.dorm_room        || ''
  } catch {
    notify('加载个人信息失败', 'error')
  }

  // 加载通知渠道
  try {
    const res = await userAPI.getChannel()
    const ch = res.data.data || {}
    channelForm.wechat_webhook = ch.wechat_webhook || ''
    channelForm.email          = ch.email          || ''
  } catch {}

  // 自动加载下拉选项
  loadDormOptions()

  // 如果缓存已加载（之前页面加载过），立即反向同步
  if (dormCacheLoaded.value) {
    reverseSyncFromProfile()
  }
})

// ---- 账号安全：修改密码 ----
const pwdFormRef = ref(null)
const pwdForm = reactive({ old_password: '', new_password: '', confirm_password: '' })
const changingPwd = ref(false)
const pwdSuccess = ref(false)

const pwdRules = [
  v => !!v || '请输入新密码',
  v => v.length >= 8 || '新密码至少 8 位',
]

const hasPwdChanges = computed(() =>
  pwdForm.old_password.length > 0 && pwdForm.new_password.length >= 8
)

const doChangePwd = async () => {
  const valid = await pwdFormRef.value?.validate()
  if (!valid?.valid) return
  if (pwdForm.new_password !== pwdForm.confirm_password) return

  changingPwd.value = true
  pwdSuccess.value = false
  try {
    await userAPI.changePassword({
      old_password: pwdForm.old_password,
      new_password: pwdForm.new_password,
    })
    pwdSuccess.value = true
    pwdForm.old_password = ''
    pwdForm.new_password = ''
    pwdForm.confirm_password = ''
    notify('密码修改成功')
  } catch (err) {
    notify(err.response?.data?.msg || '修改失败', 'error')
  } finally {
    changingPwd.value = false
  }
}

// ---- 账号安全：TOTP ----
const totpSetupDialog = ref(false)
const totpQrCode = ref('')
const totpSecret = ref('')
const totpVerifyCode = ref('')
const totpSetupError = ref('')
const totpVerifyError = ref('')
const totpSetupLoading = ref(false)
const verifyingTotp = ref(false)
const totpDisableDialog = ref(false)
const totpDisablePwd = ref('')
const totpDisableError = ref('')
const disablingTotp = ref(false)

// 从 TOTP URI 中提取密钥（用于手动输入提示）
const extractSecretFromURI = (uri) => {
  const match = uri.match(/secret=([^&]+)/)
  return match ? match[1] : ''
}

// 点击"启用两步验证"：请求 TOTP URI 并生成二维码
const openSetupTotp = async () => {
  totpSetupDialog.value = true
  totpQrCode.value = ''
  totpSecret.value = ''
  totpVerifyCode.value = ''
  totpSetupError.value = ''
  totpSetupLoading.value = true
  try {
    const res = await userAPI.totpSetup()
    const uri = res.data.data?.totp_uri || ''
    totpSecret.value = extractSecretFromURI(uri)
    totpQrCode.value = await QRCode.toDataURL(uri, {
      width: 200,
      margin: 2,
      color: { dark: '#000000', light: '#ffffff' },
    })
  } catch (err) {
    totpSetupError.value = err.response?.data?.msg || '生成 TOTP 密钥失败'
  } finally {
    totpSetupLoading.value = false
  }
}

const closeTotpSetup = () => {
  totpSetupDialog.value = false
  totpQrCode.value = ''
  totpSecret.value = ''
  totpVerifyCode.value = ''
  totpSetupError.value = ''
}

// 验证 TOTP 码并激活
const doEnableTotp = async () => {
  verifyingTotp.value = true
  totpVerifyError.value = ''
  try {
    await userAPI.totpEnable({ totp_code: totpVerifyCode.value })
    profile.totp_enabled = true
    closeTotpSetup()
    notify('两步验证已启用！请保管好您的 Authenticator 应用。')
  } catch (err) {
    totpVerifyError.value = err.response?.data?.msg || '验证码错误'
  } finally {
    verifyingTotp.value = false
  }
}

// 点击"关闭两步验证"
const openDisableTotp = () => {
  totpDisablePwd.value = ''
  totpDisableError.value = ''
  totpDisableDialog.value = true
}

// 关闭两步验证
const doDisableTotp = async () => {
  disablingTotp.value = true
  totpDisableError.value = ''
  try {
    await userAPI.totpDisable({ password: totpDisablePwd.value })
    profile.totp_enabled = false
    totpDisableDialog.value = false
    notify('两步验证已关闭')
  } catch (err) {
    totpDisableError.value = err.response?.data?.msg || '关闭失败'
  } finally {
    disablingTotp.value = false
  }
}
</script>

<style scoped>
.user-card {
  border: 1px solid rgba(var(--v-theme-primary), 0.2) !important;
  background: rgba(var(--v-theme-primary-container), 0.15) !important;
}

.scene-item {
  display: flex;
  align-items: center;
}
</style>
