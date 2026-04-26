<template>
  <v-row justify="center" class="mt-8">
    <v-col cols="12" sm="8" md="5">
      <v-card class="pa-6">
        <!-- Logo 区域 -->
        <div class="text-center mb-6">
          <div class="text-h2 mb-2">⚡</div>
          <div class="text-h5 font-weight-bold text-primary">ElectricQuery</div>
          <div class="text-body-2 text-medium-emphasis mt-1">宿舍电量查询系统</div>
        </div>

        <v-form ref="formRef" @submit.prevent="onLogin" validate-on="submit">
          <!-- 步骤一：用户名 + 密码 -->
          <template v-if="!stepTwo">
            <v-text-field
              v-model="form.username"
              label="用户名"
              prepend-inner-icon="mdi-account-circle"
              :rules="[rules.required]"
              class="mb-3"
              autofocus
            />
            <v-text-field
              v-model="form.password"
              label="密码"
              prepend-inner-icon="mdi-lock"
              :type="showPwd ? 'text' : 'password'"
              :append-inner-icon="showPwd ? 'mdi-eye-off' : 'mdi-eye'"
              @click:append-inner="showPwd = !showPwd"
              :rules="[rules.required]"
              class="mb-4"
              @keyup.enter="onLogin"
            />
          </template>

          <!-- 步骤二：TOTP 验证码 -->
          <template v-else>
            <div class="mb-4">
              <div class="text-body-2 text-medium-emphasis mb-2">
                <v-icon size="18" class="mr-1">mdi-shield-check</v-icon>
                密码验证成功，请输入两步验证码
              </div>
              <v-text-field
                v-model="form.totp_code"
                label="两步验证码"
                prepend-inner-icon="mdi-lock-clock"
                placeholder="6 位数字"
                :rules="[v => !!v || '验证码不能为空', v => /^\d{6}$/.test(v) || '验证码为 6 位数字']"
                maxlength="6"
                @keyup.enter="onLogin"
                autofocus
              />
            </div>
          </template>

          <v-btn
            type="submit"
            block
            size="large"
            color="primary"
            :loading="loading"
            class="mb-3"
          >
            <v-icon start>{{ stepTwo ? 'mdi-shield-check' : 'mdi-login' }}</v-icon>
            {{ stepTwo ? '验证并登录' : '登录' }}
          </v-btn>

          <!-- 步骤二返回按钮 -->
          <v-btn
            v-if="stepTwo"
            variant="text"
            block
            size="small"
            class="mb-3"
            @click="stepTwo = false; form.totp_code = ''"
          >
            <v-icon start>mdi-arrow-left</v-icon>
            返回上一步
          </v-btn>

          <div class="text-center text-body-2">
            还没有账号？
            <router-link to="/register" class="text-primary font-weight-medium">立即注册</router-link>
          </div>
        </v-form>
      </v-card>
    </v-col>
  </v-row>
</template>

<script setup>
import { ref, reactive, inject } from 'vue'
import { useRouter } from 'vue-router'
import { authAPI } from '@/api/index.js'

const router  = useRouter()
const notify  = inject('notify')
const userInfo = inject('userInfo')

const formRef  = ref(null)
const loading  = ref(false)
const showPwd  = ref(false)
const stepTwo  = ref(false) // 是否进入 TOTP 验证码步骤

const form = reactive({ username: '', password: '', totp_code: '' })

const rules = {
  required: v => !!v || '此字段不能为空'
}

const onLogin = async () => {
  const { valid } = await formRef.value.validate()
  if (!valid) return

  loading.value = true
  try {
    const payload = {
      username: form.username,
      password: form.password,
    }
    // 步骤二时附带 TOTP 验证码
    if (stepTwo.value) {
      payload.totp_code = form.totp_code
    }

    const res = await authAPI.login(payload)
    const data = res.data.data

    // 步骤一返回 requires_totp=true → 进入步骤二
    if (data.requires_totp) {
      stepTwo.value = true
      return
    }

    // 步骤二（或无 TOTP）成功
    const { token, user } = data
    localStorage.setItem('eq_token', token)
    localStorage.setItem('eq_user', JSON.stringify(user))
    Object.assign(userInfo, user)

    notify(`欢迎回来，${user.name || user.username}！`)
    router.push('/dashboard')
  } catch (err) {
    notify(err.response?.data?.msg || '登录失败，请检查用户名和密码', 'error')
  } finally {
    loading.value = false
  }
}
</script>
