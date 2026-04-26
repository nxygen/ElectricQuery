<template>
  <div>
    <!-- 搜索栏 -->
    <div class="d-flex gap-3 mb-4">
      <v-text-field
        v-model="userSearch"
        label="搜索用户（账号/姓名/学号）"
        prepend-inner-icon="mdi-magnify"
        density="comfortable"
        clearable
        hide-details
        class="flex-grow-1"
        @update:model-value="onSearchChange"
      />
    </div>

    <!-- 用户列表 -->
    <v-card elevation="0" :style="{ border: '1px solid rgba(var(--v-theme-outline),0.2)' }">
      <v-data-table-server
        v-model:items-per-page="pageSize"
        v-model:page="currentPage"
        :headers="userHeaders"
        :items="users"
        :items-length="userTotal"
        :loading="loadingUsers"
        loading-text="加载中..."
        no-data-text="暂无用户"
        @update:options="loadUsers"
      >
        <!-- 用户名 -->
        <template #item.username="{ item }">
          <div class="d-flex align-center">
            <v-avatar size="28" color="primary" class="mr-2">
              <span class="text-caption text-white font-weight-bold">
                {{ (item.name || item.username || 'U')[0].toUpperCase() }}
              </span>
            </v-avatar>
            <div>
              <div class="text-body-2 font-weight-medium">{{ item.username }}</div>
              <div class="text-caption text-medium-emphasis">{{ item.name || '—' }}</div>
            </div>
          </div>
        </template>

        <!-- 学号 -->
        <template #item.student_id="{ item }">
          <v-chip :color="item.student_id ? 'success' : 'grey'" size="x-small" variant="tonal">
            {{ item.student_id || '未绑定' }}
          </v-chip>
        </template>

        <!-- 两步验证 -->
        <template #item.totp_enabled="{ item }">
          <v-chip :color="item.totp_enabled ? 'error' : 'grey'" size="x-small" variant="tonal">
            <v-icon start size="12">{{ item.totp_enabled ? 'mdi-shield-lock' : 'mdi-shield-outline' }}</v-icon>
            {{ item.totp_enabled ? '已启用' : '未启用' }}
          </v-chip>
        </template>

        <!-- 宿舍 -->
        <template #item.dorm_room="{ item }">
          <div class="text-body-2">{{ item.dorm_label || item.dorm_room || '—' }}</div>
        </template>

        <!-- 注册时间 -->
        <template #item.created_at="{ item }">
          <span class="text-caption text-medium-emphasis">{{ formatDate(item.created_at) }}</span>
        </template>

        <!-- 操作 -->
        <template #item.actions="{ item }">
          <v-btn
            v-if="item.totp_enabled"
            icon="mdi-shield-lock-outline"
            size="x-small"
            variant="text"
            color="error"
            title="强制关闭两步验证"
            @click="confirmDisableTOTP(item)"
          />
          <v-btn icon="mdi-key-outline" size="x-small" variant="text" color="warning" title="重置密码" @click="confirmResetPwd(item)" />
          <v-btn icon="mdi-delete-outline" size="x-small" variant="text" color="error" @click="confirmDelete(item)" />
        </template>
      </v-data-table-server>
    </v-card>

    <!-- ====== 删除确认对话框 ====== -->
    <v-dialog v-model="deleteDialog" max-width="380">
      <v-card class="pa-5" elevation="0">
        <div class="text-subtitle-1 font-weight-bold mb-3">
          <v-icon color="error" class="mr-1">mdi-alert</v-icon>
          确认删除用户
        </div>
        <div class="text-body-2 mb-4">
          将删除用户 <strong>{{ deletingUser?.username }}</strong>，此操作不可撤销。
        </div>
        <div class="d-flex gap-2">
          <v-btn variant="text" class="flex-grow-1" @click="deleteDialog = false">取消</v-btn>
          <v-btn color="error" class="flex-grow-1" :loading="deleting" @click="doDelete">确认删除</v-btn>
        </div>
      </v-card>
    </v-dialog>

    <!-- ====== 重置密码对话框 ====== -->
    <v-dialog v-model="resetPwdDialog" max-width="420">
      <v-card class="pa-5" elevation="0">
        <div class="text-subtitle-1 font-weight-bold mb-3">
          <v-icon color="warning" class="mr-1">mdi-key</v-icon>
          重置用户密码
        </div>
        <div class="text-body-2 mb-3">
          将为用户 <strong>{{ resettingUser?.username }}</strong> 生成新的随机密码，请将密码告知该用户。
        </div>
        <v-alert v-if="resetSuccess" type="success" variant="tonal" density="compact" class="mb-3">
          <div class="font-weight-bold mb-1">密码已重置</div>
          <div class="text-caption">请通过安全渠道（当面或加密消息）将新密码告知用户，并提醒其登录后立即修改。</div>
        </v-alert>
        <div class="d-flex gap-2">
          <v-btn variant="text" class="flex-grow-1" @click="closeResetPwd">关闭</v-btn>
          <v-btn v-if="!resetSuccess" color="warning" class="flex-grow-1" :loading="resettingPwd" @click="doResetPwd">
            生成新密码
          </v-btn>
        </div>
      </v-card>
    </v-dialog>

    <!-- ====== 强制关闭两步验证对话框 ====== -->
    <v-dialog v-model="disableTOTPDialog" max-width="400">
      <v-card class="pa-5" elevation="0">
        <div class="text-subtitle-1 font-weight-bold mb-3">
          <v-icon color="error" class="mr-1">mdi-shield-lock-outline</v-icon>
          强制关闭两步验证
        </div>
        <div class="text-body-2 mb-4">
          将强制关闭用户 <strong>{{ disablingTOTPUser?.username }}</strong> 的两步验证。<br/>
          该用户下次登录时将不再需要输入验证码。此操作不可撤销。
        </div>
        <div class="d-flex gap-2">
          <v-btn variant="text" class="flex-grow-1" @click="disableTOTPDialog = false">取消</v-btn>
          <v-btn color="error" class="flex-grow-1" :loading="disablingTOTP" @click="doDisableTOTP">
            确认关闭
          </v-btn>
        </div>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup>
import { ref, inject, onMounted } from 'vue'
import { adminAPI } from '@/api/index.js'

const adminAuthed = inject('adminAuthed')
const notify = inject('notify')

const users       = ref([])
const userTotal   = ref(0)
const currentPage = ref(1)
const pageSize    = ref(20)
const userSearch  = ref('')
const loadingUsers = ref(false)

const userHeaders = [
  { title: '用户', key: 'username',    sortable: false, minWidth: '160' },
  { title: '学号', key: 'student_id',  sortable: false },
  { title: '两步验证', key: 'totp_enabled', sortable: false },
  { title: '宿舍', key: 'dorm_room',   sortable: false },
  { title: '班级', key: 'class',       sortable: false },
  { title: '注册时间', key: 'created_at', sortable: false },
  { title: '操作', key: 'actions',    sortable: false, width: '80' },
]

let searchTimer = null
const onSearchChange = () => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    currentPage.value = 1
    loadUsers()
  }, 400)
}

const loadUsers = async (opts = {}) => {
  if (!adminAuthed.value) return
  if (opts.page) currentPage.value = opts.page
  if (opts.itemsPerPage) pageSize.value = opts.itemsPerPage
  loadingUsers.value = true
  try {
    const res = await adminAPI.listUsers({ page: currentPage.value, size: pageSize.value, search: userSearch.value || undefined })
    const d = res.data.data
    users.value    = d.users  || []
    userTotal.value = d.total || 0
  } catch {
    notify('加载用户列表失败', 'error')
  } finally {
    loadingUsers.value = false
  }
}

// ---- 删除用户 ----
const deleteDialog = ref(false)
const deletingUser  = ref(null)
const deleting      = ref(false)

const confirmDelete = (user) => { deletingUser.value = user; deleteDialog.value = true }

const doDelete = async () => {
  if (!deletingUser.value) return
  deleting.value = true
  try {
    await adminAPI.deleteUser(deletingUser.value.id)
    notify(`用户 ${deletingUser.value.username} 已删除`)
    deleteDialog.value = false
    loadUsers()
  } catch (err) {
    notify(err.response?.data?.msg || '删除失败', 'error')
  } finally {
    deleting.value = false
  }
}

// ---- 重置密码 ----
const resetPwdDialog = ref(false)
const resettingUser  = ref(null)
const resettingPwd   = ref(false)
const resetSuccess   = ref(false)

const confirmResetPwd = (user) => { resettingUser.value = user; resetSuccess.value = false; resetPwdDialog.value = true }

const doResetPwd = async () => {
  if (!resettingUser.value) return
  resettingPwd.value = true
  try {
    await adminAPI.resetPassword(resettingUser.value.id)
    resetSuccess.value = true
  } catch (err) {
    notify(err.response?.data?.msg || '重置密码失败', 'error')
    resetPwdDialog.value = false
  } finally {
    resettingPwd.value = false
  }
}

const closeResetPwd = () => { resetPwdDialog.value = false; resetSuccess.value = false }

// ---- 强制关闭两步验证 ----
const disableTOTPDialog    = ref(false)
const disablingTOTPUser     = ref(null)
const disablingTOTP         = ref(false)

const confirmDisableTOTP = (user) => { disablingTOTPUser.value = user; disableTOTPDialog.value = true }

const doDisableTOTP = async () => {
  if (!disablingTOTPUser.value) return
  disablingTOTP.value = true
  try {
    await adminAPI.disableTOTP(disablingTOTPUser.value.id)
    notify(`已关闭 ${disablingTOTPUser.value.username} 的两步验证`)
    disableTOTPDialog.value = false
    loadUsers()
  } catch (err) {
    notify(err.response?.data?.msg || '操作失败', 'error')
  } finally {
    disablingTOTP.value = false
  }
}

const formatDate = (iso) => { if (!iso) return '—'; return new Date(iso).toLocaleDateString('zh-CN') }

onMounted(() => { if (adminAuthed.value) loadUsers() })
</script>
