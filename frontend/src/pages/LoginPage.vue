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
          <v-text-field
            v-model="form.student_id"
            label="学号"
            prepend-inner-icon="mdi-card-account-details"
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

          <v-btn
            type="submit"
            block
            size="large"
            color="primary"
            :loading="loading"
            class="mb-3"
          >
            <v-icon start>mdi-login</v-icon>
            登录
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

const form = reactive({ student_id: '', password: '' })

const rules = {
  required: v => !!v || '此字段不能为空'
}

const onLogin = async () => {
  const { valid } = await formRef.value.validate()
  if (!valid) return

  loading.value = true
  try {
    const res = await authAPI.login(form)
    const { token, user } = res.data.data

    localStorage.setItem('eq_token', token)
    localStorage.setItem('eq_user', JSON.stringify(user))
    Object.assign(userInfo, user)

    notify(`欢迎回来，${user.name || user.student_id}！`)
    router.push('/dashboard')
  } catch (err) {
    notify(err.response?.data?.msg || '登录失败，请检查学号和密码', 'error')
  } finally {
    loading.value = false
  }
}
</script>
