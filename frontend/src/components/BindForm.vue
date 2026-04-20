<template>
  <el-form :model="form" :rules="rules" ref="formRef" label-position="top" >
    <el-form-item label="宿舍号（如 110132）" prop="dorm">
      <el-input v-model="form.dorm" placeholder="楼层+房间（例：110132）" />
    </el-form-item>

    <el-form-item label="企业微信用户 ID（可选）" prop="user_id">
      <el-input v-model="form.user_id" placeholder="企业微信 user id 或 openid" />
    </el-form-item>

    <el-form-item label="邮箱（可选）" prop="email">
      <el-input v-model="form.email" placeholder="用于接收邮件通知" />
    </el-form-item>

    <el-form-item>
      <el-button type="primary" @click="submit">立即绑定</el-button>
      <el-button @click="reset">重置</el-button>
    </el-form-item>
  </el-form>
</template>

<script>
import api from '../api'
import { ref } from 'vue'

export default {
  name: 'BindForm',
  setup (props, { emit }) {
    const formRef = ref(null)
    const form = ref({ dorm: '', user_id: '', email: '' })
    const rules = {
      dorm: [ { required: true, message: '请输入宿舍号', trigger: 'blur' } ],
      email: [ { type: 'email', message: '邮箱格式不正确', trigger: 'blur' } ]
    }

    const submit = async () => {
      try {
        await formRef.value.validate()
      } catch (e) {
        return
      }

      const payload = {
        dorm: form.value.dorm,
        user_id: form.value.user_id || undefined,
        email: form.value.email || undefined
      }

      const res = await api.bind(payload)
      if (res && res.ok) {
        emit('bound')
        form.value = { dorm: '', user_id: '', email: '' }
        formRef.value.clearValidate()
      }
    }

    const reset = () => {
      form.value = { dorm: '', user_id: '', email: '' }
      formRef.value.clearValidate()
    }

    return { formRef, form, rules, submit, reset }
  }
}
</script>

<style scoped>
</style>
