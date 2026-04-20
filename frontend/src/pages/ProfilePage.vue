<template>
  <div>
    <!-- ====== 学号绑定卡片 ====== -->
    <v-card
      class="mb-5 pa-5 student-id-card"
      :class="profile.student_id ? 'card-bound' : 'card-unbound'"
      elevation="0"
    >
      <div class="d-flex align-center mb-4">
        <v-avatar
          :color="profile.student_id ? 'success-container' : 'error-container'"
          size="44"
        >
          <v-icon :color="profile.student_id ? 'success' : 'error'" size="22">
            {{ profile.student_id ? 'mdi-check-circle' : 'mdi-alert-circle' }}
          </v-icon>
        </v-avatar>
        <div class="ml-3">
          <div class="text-subtitle-1 font-weight-bold">学号绑定</div>
          <div class="text-caption text-medium-emphasis">
            {{ profile.student_id
              ? `已绑定学号：${profile.student_id}`
              : '您尚未绑定学号，登录时需要使用学号' }}
          </div>
        </div>
      </div>

      <!-- 已绑定时：显示只读学号 + 修改按钮 -->
      <template v-if="profile.student_id">
        <div class="d-flex align-center">
          <v-text-field
            :model-value="profile.student_id"
            label="学号"
            prepend-inner-icon="mdi-identifier"
            readonly
            variant="outlined"
            density="comfortable"
            class="mr-3 flex-grow-1"
          />
          <v-btn
            color="primary"
            variant="tonal"
            :loading="bindingStudentId"
            @click="showStudentIdDialog = true"
          >
            <v-icon start>mdi-pencil</v-icon>
            修改
          </v-btn>
        </div>
      </template>

      <!-- 未绑定时：填写学号表单 -->
      <template v-else>
        <v-form ref="studentIdFormRef" @submit.prevent="bindStudentId">
          <div class="d-flex align-center gap-2">
            <v-text-field
              v-model="studentIdInput"
              label="请输入学号"
              prepend-inner-icon="mdi-identifier"
              placeholder="例如：2023123456"
              :rules="[v => !!v || '学号不能为空', v => v.length >= 6 || '学号至少6位']"
              class="flex-grow-1"
              autofocus
              @keyup.enter="bindStudentId"
            />
            <v-btn
              type="submit"
              color="primary"
              size="large"
              :loading="bindingStudentId"
              class="bind-btn"
            >
              <v-icon start>mdi-check</v-icon>
              绑定
            </v-btn>
          </div>
        </v-form>
        <div v-if="studentIdError" class="text-caption text-error mt-1 ml-2">
          <v-icon size="14" class="mr-1">mdi-alert</v-icon>
          {{ studentIdError }}
        </div>
      </template>
    </v-card>

    <!-- ====== 基本信息卡片 ====== -->
    <v-card class="mb-5 pa-5" elevation="0">
      <div class="text-subtitle-1 font-weight-bold mb-4">
        <v-icon class="mr-1 text-primary" size="18">mdi-account</v-icon>
        基本信息
      </div>

      <v-form ref="formRef" @submit.prevent="saveProfile">
        <v-text-field
          v-model="form.name"
          label="姓名"
          prepend-inner-icon="mdi-account"
          placeholder="方便通知时称呼您"
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
        <div class="text-subtitle-1 font-weight-bold mb-4">
          <v-icon class="mr-1 text-primary" size="18">mdi-home-city</v-icon>
          宿舍信息
        </div>

        <!-- 宿舍选择：楼栋下拉 + 房间号输入 -->
        <v-row dense class="mb-2">
          <!-- 楼栋选择 -->
          <v-col cols="12" sm="5">
            <v-select
              v-model="form.building"
              label="宿舍楼"
              prepend-inner-icon="mdi-office-building"
              :items="buildingOptions"
              placeholder="选择楼栋"
              clearable
              @update:model-value="onBuildingChange"
            />
          </v-col>
          <!-- 楼层（自动推断，可手动修改）-->
          <v-col cols="12" sm="3">
            <v-text-field
              v-model="form.floor"
              label="楼层"
              prepend-inner-icon="mdi-stairs"
              placeholder="如：3"
              clearable
            />
          </v-col>
          <!-- 房间号 -->
          <v-col cols="12" sm="4">
            <v-text-field
              v-model="form.room"
              label="房间号"
              prepend-inner-icon="mdi-door"
              placeholder="如：1301"
              clearable
            />
          </v-col>
        </v-row>

        <!-- 完整宿舍号展示 + 实时校验 -->
        <v-text-field
          :model-value="fullDormRoom"
          label="完整宿舍号"
          prepend-inner-icon="mdi-text-box"
          readonly
          variant="outlined"
          density="comfortable"
          hint="上方选择楼栋+房间号后自动生成，或手动输入完整格式"
          persistent-hint
          class="mb-1"
        >
          <template #append-inner>
            <v-btn
              icon="mdi-refresh"
              variant="text"
              size="small"
              :loading="validatingDorm"
              title="点击校验此宿舍是否存在"
              @click="validateDormRoom"
            />
          </template>
        </v-text-field>

        <!-- 宿舍号提示 -->
        <div
          v-if="dormHint"
          class="text-caption text-medium-emphasis mb-3 ml-2"
        >
          <v-icon size="12" class="mr-1">mdi-information-outline</v-icon>
          {{ dormHint }}
        </div>

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

        <!-- 手动输入完整宿舍号（备选）-->
        <v-expansion-panels variant="accordion" class="mb-4">
          <v-expansion-panel>
            <v-expansion-panel-title class="text-caption">
              <v-icon size="14" class="mr-1">mdi-keyboard</v-icon>
              手动输入电费宿舍号（覆盖上方选择）
            </v-expansion-panel-title>
            <v-expansion-panel-text>
              <v-text-field
                v-model="form.dorm_room"
                label="电费宿舍号"
                prepend-inner-icon="mdi-text-short"
                placeholder="如：140328 或 01-0101-010101"
                hint="支持多种格式，系统会自动解析"
                persistent-hint
                class="mt-2"
                @blur="onDormRoomManualInput"
              />
            </v-expansion-panel-text>
          </v-expansion-panel>
        </v-expansion-panels>

        <!-- 水费宿舍号（独立绑定，适用于水电分房间的楼栋）-->
        <v-card variant="outlined" class="mb-4 pa-4" rounded="lg">
          <div class="d-flex align-center mb-3">
            <v-icon size="18" class="mr-2 text-info">mdi-water</v-icon>
            <span class="text-subtitle-2 font-weight-bold">水费宿舍号</span>
            <v-chip class="ml-2" size="x-small" color="info" variant="tonal">非必填</v-chip>
            <span class="text-caption text-medium-emphasis ml-2">C13/C14 楼水电同页，只需绑定电费宿舍即可同时查询水电</span>
          </div>

          <!-- 水费宿舍三段式下拉 -->
          <v-row dense class="mb-2">
            <v-col cols="12" sm="5">
              <v-select
                v-model="form.water_building"
                label="水费楼栋"
                prepend-inner-icon="mdi-office-building"
                :items="buildingOptions"
                placeholder="选择楼栋"
                clearable
                @update:model-value="onWaterBuildingChange"
              />
            </v-col>
            <v-col cols="12" sm="3">
              <v-text-field
                v-model="form.water_floor"
                label="楼层"
                prepend-inner-icon="mdi-stairs"
                placeholder="如：3"
                clearable
              />
            </v-col>
            <v-col cols="12" sm="4">
              <v-text-field
                v-model="form.water_room"
                label="房间号"
                prepend-inner-icon="mdi-door"
                placeholder="如：1301"
                clearable
              />
            </v-col>
          </v-row>

          <!-- 完整水费宿舍号展示 -->
          <v-text-field
            :model-value="fullWaterDormRoom"
            label="完整水费宿舍号"
            prepend-inner-icon="mdi-text-box"
            readonly
            variant="outlined"
            density="comfortable"
            hint="上方选择后自动生成，或手动输入覆盖"
            persistent-hint
            class="mb-2"
          />

          <!-- 水费宿舍号提示 -->
          <div class="text-caption text-medium-emphasis mb-2 ml-2">
            <v-icon size="12" class="mr-1">mdi-information-outline</v-icon>
            水电分房间的楼栋（如 C13/C01 等）需单独填写水费宿舍号
          </div>

          <!-- 校验结果 -->
          <v-expand-transition>
            <v-alert
              v-if="waterValidateResult"
              :type="waterValid ? 'success' : 'error'"
              variant="tonal"
              density="compact"
              rounded="lg"
              class="mb-2"
              :icon="waterValid ? 'mdi-check-circle' : 'mdi-alert-circle'"
            >
              {{ waterValidateResult }}
            </v-alert>
          </v-expand-transition>

          <!-- 手动输入完整水费宿舍号（备选）-->
          <v-expansion-panels variant="accordion">
            <v-expansion-panel>
              <v-expansion-panel-title class="text-caption">
                <v-icon size="14" class="mr-1">mdi-keyboard</v-icon>
                手动输入水费宿舍号（覆盖上方选择）
              </v-expansion-panel-title>
              <v-expansion-panel-text>
                <v-text-field
                  v-model="form.water_dorm_room"
                  label="水费宿舍号"
                  prepend-inner-icon="mdi-text-short"
                  placeholder="如：C13-1301水 或 01-0101-010101"
                  hint="支持多种格式，系统会自动解析"
                  persistent-hint
                  class="mt-2"
                  @blur="onWaterDormRoomManualInput"
                />
              </v-expansion-panel-text>
            </v-expansion-panel>
          </v-expansion-panels>
        </v-card>

        <v-btn
          type="submit"
          color="primary"
          size="large"
          block
          :loading="saving"
          :disabled="!hasChanges"
        >
          <v-icon start>mdi-content-save</v-icon>
          保存信息
        </v-btn>
      </v-form>
    </v-card>

    <!-- ====== 账号信息卡片 ====== -->
    <v-card variant="outlined" class="pa-5" elevation="0">
      <div class="text-subtitle-2 font-weight-bold mb-3">
        <v-icon class="mr-1 text-medium-emphasis" size="16">mdi-shield-account</v-icon>
        账号信息
      </div>
      <div class="text-body-2 text-medium-emphasis mb-1">
        注册时间：{{ formatDate(profile.created_at) }}
      </div>
      <div class="text-body-2 text-medium-emphasis">
        用户 ID：{{ profile.id }}
      </div>
    </v-card>

    <!-- ====== 修改学号对话框 ====== -->
    <v-dialog v-model="showStudentIdDialog" max-width="400">
      <v-card class="pa-5" elevation="0">
        <div class="text-h6 font-weight-bold mb-4">修改学号</div>
        <v-form ref="editStudentIdFormRef" @submit.prevent="updateStudentId">
          <v-text-field
            v-model="studentIdInput"
            label="新学号"
            prepend-inner-icon="mdi-identifier"
            :rules="[v => !!v || '学号不能为空', v => v.length >= 6 || '学号至少6位']"
            class="mb-4"
          />
          <div class="d-flex gap-2">
            <v-btn
              variant="text"
              class="flex-grow-1"
              @click="showStudentIdDialog = false"
            >
              取消
            </v-btn>
            <v-btn
              type="submit"
              color="primary"
              class="flex-grow-1"
              :loading="bindingStudentId"
            >
              确认修改
            </v-btn>
          </div>
        </v-form>
      </v-card>
    </v-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, inject, watch } from 'vue'
import { userAPI } from '@/api/index.js'

const notify = inject('notify')

const profile = reactive({})
const form    = reactive({
  name: '', class: '',
  building: '', floor: '', room: '', dorm_room: '',
  // 水费宿舍（独立三段式）
  water_building: '', water_floor: '', water_room: '', water_dorm_room: '',
})
const saving = ref(false)

// 学号绑定
const studentIdInput      = ref('')
const studentIdError      = ref('')
const bindingStudentId    = ref(false)
const studentIdFormRef    = ref(null)
const editStudentIdFormRef = ref(null)
const showStudentIdDialog  = ref(false)

// 宿舍校验
const validatingDorm       = ref(false)
const dormValid           = ref(false)
const dormValidateResult  = ref('')

// 表单引用
const formRef = ref(null)

// 楼栋选项（统一用纯数字，如 "01" "14"）
const buildingOptions = [
  { title: '01 楼',  value: '01'  },
  { title: '02 楼',  value: '02'  },
  { title: '03 楼',  value: '03'  },
  { title: '04 楼',  value: '04'  },
  { title: '05 楼',  value: '05'  },
  { title: '06 楼',  value: '06'  },
  { title: '07 楼',  value: '07'  },
  { title: '08 楼',  value: '08'  },
  { title: '09 楼',  value: '09'  },
  { title: '10 楼',  value: '10'  },
  { title: '11 楼',  value: '11'  },
  { title: '12 楼',  value: '12'  },
  { title: '13 楼',  value: '13'  },
  { title: '14 楼',  value: '14'  },
]

// 楼栋变化时自动推断楼层（电费）
const onBuildingChange = () => {
  if (form.room && form.building) {
    const floorMatch = form.room.match(/^(\d)(\d{2,})$/)
    if (floorMatch) {
      form.floor = floorMatch[1]
    }
  }
}

// 楼栋变化时自动推断楼层（水费）
const onWaterBuildingChange = () => {
  if (form.water_room && form.water_building) {
    const floorMatch = form.water_room.match(/^(\d)(\d{2,})$/)
    if (floorMatch) {
      form.water_floor = floorMatch[1]
    }
  }
}

// 完整宿舍号（自动拼接，含楼层）
// 格式：楼栋去掉C前缀 + 0 + 楼层 + 房间号 = 纯6位数字，如 "C14"+"3"+"28" → "140328"
const fullDormRoom = computed(() => {
  if (form.dorm_room) return form.dorm_room
  if (form.building && form.room) {
    if (form.floor) {
      // 有楼层：去掉C前缀，补0，例 C14-3-28 → 140328
      const bldNum = form.building.replace(/^C/i, '')
      return bldNum + '0' + form.floor + form.room
    }
    // 无楼层：去掉C前缀，例 C13-1301 → 13-0101-010101 (旧格式可能带电/水标识)
    return form.building.replace(/^C/i, '') + '-' + form.room
  }
  return ''
})

// 完整水费宿舍号（自动拼接）
// 格式：楼栋去掉C前缀 + 0 + 楼层 + 房间号 = 纯6位数字
const fullWaterDormRoom = computed(() => {
  if (form.water_dorm_room) return form.water_dorm_room
  if (form.water_building && form.water_room) {
    if (form.water_floor) {
      const bldNum = form.water_building.replace(/^C/i, '')
      return bldNum + '0' + form.water_floor + form.water_room
    }
    return form.water_building.replace(/^C/i, '') + '-' + form.water_room
  }
  return ''
})

// 宿舍提示
const dormHint = computed(() => {
  if (!fullDormRoom.value) return '请选择楼栋和房间号，或手动输入完整格式'
  const d = fullDormRoom.value
  if (d.includes('电') || d.includes('水')) {
    return '水电分开计费的宿舍，请在下方单独绑定水费宿舍号'
  }
  // C13/C14 水电同页
  if (/C13|C14/i.test(d)) {
    return 'C13/C14 水电同页，水量随电费查询一并返回，无需单独绑定水费'
  }
  return '格式示例：140328（水电同页）或 13-0101-010101（水电分房）'
})

// 手动输入完整宿舍号时，解析楼栋和房间（电费）
const onDormRoomManualInput = () => {
  if (!form.dorm_room) return
  const parts = form.dorm_room.split('-')
  if (parts.length >= 2) {
    form.building = parts[0]
    const roomPart = parts[parts.length - 1]
    form.room = roomPart
    // 尝试提取楼层
    const floorMatch = roomPart.match(/^(\d)(\d{2,})$/)
    if (floorMatch) {
      form.floor = floorMatch[1]
    } else {
      form.floor = ''
    }
  }
}

// 手动输入完整水费宿舍号时，解析楼栋和房间
const onWaterDormRoomManualInput = () => {
  if (!form.water_dorm_room) return
  const parts = form.water_dorm_room.split('-')
  if (parts.length >= 2) {
    form.water_building = parts[0]
    const roomPart = parts[parts.length - 1]
    form.water_room = roomPart
    const floorMatch = roomPart.match(/^(\d)(\d{2,})$/)
    if (floorMatch) {
      form.water_floor = floorMatch[1]
    } else {
      form.water_floor = ''
    }
  }
}

// 是否有未保存的变更
const hasChanges = computed(() => {
  return form.name            !== (profile.name             || '')
      || form.class           !== (profile.class           || '')
      || form.building        !== (profile.building        || '')
      || form.dorm_room       !== (profile.dorm_room      || '')
      || form.water_building  !== (profile.water_building || '')
      || form.water_dorm_room !== (profile.water_dorm_room || '')
})

// 校验宿舍号（调用后端真实校验）
const validateDormRoom = async () => {
  const dorm = fullDormRoom.value || form.dorm_room
  if (!dorm) {
    dormValidateResult.value = '请先填写电费宿舍号'
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
    if (!data.valid) {
      notify(data.message, 'error')
    }
  } catch (err) {
    dormValid.value = false
    dormValidateResult.value = err.response?.data?.msg || '校验失败，请检查宿舍号格式'
  } finally {
    validatingDorm.value = false
  }
}

// 校验水费宿舍号
const waterValid = ref(false)
const waterValidateResult = ref('')
const validateWaterDormRoom = async () => {
  const dorm = fullWaterDormRoom.value || form.water_dorm_room
  if (!dorm) {
    waterValidateResult.value = '请先填写水费宿舍号'
    waterValid.value = false
    return
  }

  validatingDorm.value = true
  waterValidateResult.value = ''
  try {
    const res = await userAPI.validateDorm({ dorm_room: dorm })
    const data = res.data.data
    waterValid.value = data.valid
    waterValidateResult.value = data.message
  } catch (err) {
    waterValid.value = false
    waterValidateResult.value = err.response?.data?.msg || '校验失败'
  } finally {
    validatingDorm.value = false
  }
}

// 绑定学号
const bindStudentId = async () => {
  const { valid } = await studentIdFormRef.value?.validate()
  if (!valid) return

  bindingStudentId.value = true
  studentIdError.value = ''
  try {
    const res = await userAPI.bindStudentId({ student_id: studentIdInput.value })
    Object.assign(profile, res.data.data)
    // 更新全局 userInfo
    const storedUser = JSON.parse(localStorage.getItem('eq_user') || '{}')
    Object.assign(storedUser, res.data.data)
    localStorage.setItem('eq_user', JSON.stringify(storedUser))
    notify('学号绑定成功！')
  } catch (err) {
    const msg = err.response?.data?.msg || '绑定失败'
    if (msg.includes('已被')) {
      studentIdError.value = msg
    } else {
      notify(msg, 'error')
    }
  } finally {
    bindingStudentId.value = false
  }
}

// 修改学号（对话框确认）
const updateStudentId = async () => {
  const { valid } = await editStudentIdFormRef.value?.validate()
  if (!valid) return

  bindingStudentId.value = true
  try {
    const res = await userAPI.bindStudentId({ student_id: studentIdInput.value })
    Object.assign(profile, res.data.data)
    const storedUser = JSON.parse(localStorage.getItem('eq_user') || '{}')
    Object.assign(storedUser, res.data.data)
    localStorage.setItem('eq_user', JSON.stringify(storedUser))
    showStudentIdDialog.value = false
    notify('学号已更新！')
  } catch (err) {
    notify(err.response?.data?.msg || '修改失败', 'error')
  } finally {
    bindingStudentId.value = false
  }
}

// 保存个人信息
const saveProfile = async () => {
  saving.value = true
  try {
    const payload = {
      name:             form.name,
      class:            form.class,
      building:         fullDormRoom.value ? form.building : (form.dorm_room ? '' : ''),
      dorm_room:        fullDormRoom.value || form.dorm_room,
      water_dorm_room:  fullWaterDormRoom.value || form.water_dorm_room,
    }
    const res = await userAPI.updateProfile(payload)
    Object.assign(profile, res.data.data)
    const storedUser = JSON.parse(localStorage.getItem('eq_user') || '{}')
    Object.assign(storedUser, res.data.data)
    localStorage.setItem('eq_user', JSON.stringify(storedUser))
    notify('个人信息已保存')
  } catch (err) {
    notify(err.response?.data?.msg || '保存失败', 'error')
  } finally {
    saving.value = false
  }
}

const formatDate = (iso) => {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('zh-CN')
}

onMounted(async () => {
  try {
    const res = await userAPI.getProfile()
    Object.assign(profile, res.data.data)
    form.name             = profile.name             || ''
    form.class            = profile.class           || ''
    form.building         = profile.building        || ''
    form.dorm_room        = profile.dorm_room      || ''
    form.water_dorm_room  = profile.water_dorm_room || ''
    studentIdInput.value  = profile.student_id || ''
    // 从电费宿舍号解析楼栋和房间
    if (profile.dorm_room) {
      onDormRoomManualInput()
    }
    // 从水费宿舍号解析楼栋和房间
    if (profile.water_dorm_room) {
      onWaterDormRoomManualInput()
    }
  } catch (err) {
    notify('加载个人信息失败', 'error')
  }
})
</script>

<style scoped>
.student-id-card {
  border: 1px solid transparent;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}
.card-unbound {
  border-color: rgba(var(--v-theme-error), 0.3) !important;
  background: rgba(var(--v-theme-error-container), 0.15) !important;
}
.card-bound {
  border-color: rgba(var(--v-theme-success), 0.3) !important;
  background: rgba(var(--v-theme-success-container), 0.1) !important;
}

.bind-btn {
  min-width: 100px !important;
}
</style>
