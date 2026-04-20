<template>
  <div>
    <div style="margin-bottom:12px; display:flex; gap:8px; align-items:center;">
      <div style="color:#666">管理员 Token: <strong>{{ tokenStatus }}</strong></div>
      <div style="margin-left:auto;color:#999;font-size:12px">（开发模式下 token secret 可写入 vite.config.js 的 VITE_ADMIN_TOKEN_SECRET，前端会自动生成短期 token）</div>
    </div>

    <el-table :data="bindings" stripe style="width:100%">
      <el-table-column prop="user_id" label="用户ID" width="160" />
      <el-table-column prop="dorm" label="宿舍" width="120" />
      <el-table-column prop="email" label="邮箱" />
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button type="danger" size="small" @click="unbind(row)">解绑</el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script>
import api from '../api'
import { ref, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'

export default {
  name: 'BindList',
  setup () {
    const bindings = ref([])
    // token secret 来自 vite.config 中注入的 VITE_ADMIN_TOKEN_SECRET
    const tokenSecret = import.meta.env.VITE_ADMIN_TOKEN_SECRET || ''
    const tokenStatus = computed(() => tokenSecret ? '已配置（自动生成）' : '未设置')
    
    const load = async () => {
      const res = await api.bindings()
      if (res && res.ok) bindings.value = res.bindings || []
      else if (res && res.ok === false) {
        // 可能是未授权
        ElMessage.error(res.error || '获取绑定列表失败')
      }
    }

    const unbind = async (row) => {
      await api.unbind({ user_id: row.user_id, dorm: row.dorm })
      load()
    }

    onMounted(() => {
      // token 会在 axios 拦截器中自动生成并注入，直接加载列表
      load()
    })

    return { bindings, load, unbind, tokenStatus }
  }
}
</script>

<style scoped>
</style>
