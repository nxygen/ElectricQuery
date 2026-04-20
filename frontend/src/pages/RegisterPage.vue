<template>
  <v-row justify="center" class="mt-4">
    <v-col cols="12" sm="8" md="5" lg="4">

      <!-- Logo + 标题 -->
      <div class="text-center mb-6">
        <v-avatar color="primary-container" size="72" class="mb-4 elevation-0">
          <v-icon color="primary" size="36">mdi-lightning-bolt</v-icon>
        </v-avatar>
        <div class="text-h5 font-weight-bold text-on-surface">注册账号</div>
        <div class="text-body-2 text-medium-emphasis mt-1">注册后需在「个人信息」页绑定学号</div>
      </div>

      <!-- 注册表单卡片 -->
      <v-card class="pa-5" elevation="0">
        <v-form ref="formRef" @submit.prevent="onRegister" validate-on="submit">

          <!-- 姓名（选填） -->
          <v-text-field
            v-model="form.name"
            label="姓名（选填）"
            prepend-inner-icon="mdi-account"
            placeholder="方便通知时称呼您"
            class="mb-3"
            autofocus
          />

          <!-- 密码 -->
          <v-text-field
            v-model="form.password"
            label="密码"
            prepend-inner-icon="mdi-lock"
            :type="showPwd ? 'text' : 'password'"
            :append-inner-icon="showPwd ? 'mdi-eye-off' : 'mdi-eye'"
            @click:append-inner="showPwd = !showPwd"
            :rules="[rules.required, rules.minLen6]"
            hint="至少 6 位，建议包含字母和数字"
            persistent-hint
            class="mb-3"
          />

          <!-- 确认密码 -->
          <v-text-field
            v-model="form.confirm"
            label="确认密码"
            prepend-inner-icon="mdi-lock-check"
            :type="showPwd ? 'text' : 'password'"
            :rules="[rules.required, rules.confirmMatch]"
            class="mb-5"
            @keyup.enter="onRegister"
          />

          <v-btn
            type="submit"
            block
            size="large"
            color="primary"
            :loading="loading"
            class="mb-3"
          >
            <v-icon start>mdi-account-plus</v-icon>
            创建账号
          </v-btn>

          <div class="text-center text-body-2">
            已有账号？
            <router-link to="/login" class="text-primary font-weight-medium">返回登录</router-link>
          </div>
        </v-form>
      </v-card>

      <!-- 底部说明 -->
      <v-alert
        type="info"
        variant="tonal"
        density="compact"
        class="mt-4"
        rounded="lg"
        icon="mdi-information-outline"
      >
        <span class="text-body-2">
          学号将作为登录账号使用，绑定后可使用学号登录。
          <router-link to="/profile" class="text-primary ml-1">前往绑定 →</router-link>
        </span>
      </v-alert>

    </v-col>
  </v-row>
</template>

<script setup>
import { ref, reactive, inject } from 'vue'
import { useRouter } from 'vue-router'
import { authAPI } from '@/api/index.js'

const router = useRouter()
const notify = inject('notify')

const formRef = ref(null)
const loading = ref(false)
const showPwd = ref(false)

const form = reactive({
  name:     '',
  password: '',
  confirm:  ''
})

const rules = {
  required:     v => !!v || '此字段不能为空',
  minLen6:      v => (v && v.length >= 6) || '至少需要 6 个字符',
  confirmMatch: v => v === form.password || '两次密码不一致',
}

const onRegister = async () => {
  const { valid } = await formRef.value.validate()
  if (!valid) return

  loading.value = true
  try {
    await authAPI.register({
      name:     form.name,
      password: form.password,
    })
    notify('注册成功，请登录！', 'success')
    router.push('/login')
  } catch (err) {
    notify(err.response?.data?.msg || '注册失败', 'error')
  } finally {
    loading.value = false
  }
}
</script>
