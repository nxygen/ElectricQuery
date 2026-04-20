<template>
  <div id="app" class="app-root">
    <el-container>
      <el-header class="app-header">
        <div class="logo">ElectricQuery</div>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
      <el-footer class="app-footer">© ElectricQuery</el-footer>
    </el-container>
  </div>
</template>

<script>
export default {
  name: 'App'
}
</script>

<style scoped>
.app-root { min-height: 100vh; display:flex; flex-direction:column; }
.app-header { background: #fff; border-bottom: 1px solid #eaeef2; display:flex; align-items:center; padding: 0 18px; }
.logo { font-weight: 700; font-size: 18px; }
.app-footer { text-align:center; color:#999; padding:12px 0; }
</style>
