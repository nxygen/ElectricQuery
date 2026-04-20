<template>
  <div class="page-bind">
    <el-row :gutter="20">
      <el-col :span="10">
        <el-card>
          <h3>绑定宿舍 / 企业微信 / 邮箱</h3>
          <bind-form @bound="onBound" />
        </el-card>
      </el-col>
      <el-col :span="14">
        <el-card>
          <h3>已绑定列表</h3>
          <bind-list ref="listRef" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script>
import { ref } from 'vue'
import BindForm from '../components/BindForm.vue'
import BindList from '../components/BindList.vue'
import { ElMessage } from 'element-plus'

export default {
  components: { BindForm, BindList },
  setup () {
    const listRef = ref(null)
    
    const onBound = () => {
      // 绑定成功后刷新列表
      if (listRef.value && listRef.value.load) {
        listRef.value.load()
        ElMessage.success('绑定成功！')
      }
    }

    return { listRef, onBound }
  }
}
</script>

<style scoped>
.page-bind { padding: 18px; }
h3 { margin: 0 0 12px 0; }
</style>
