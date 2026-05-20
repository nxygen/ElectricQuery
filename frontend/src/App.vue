<template>
  <!-- public-route/admin-unauthed 由 CSS 在 JS 执行前就隐藏导航，无闪烁 -->
  <v-app :class="{ 'public-route': routeLayout === 'public' || routeLayout === 'admin', 'admin-unauthed': navType === 'admin' && !adminAuthed }">

  <!-- ====== 侧边导航 Drawer ====== -->
  <v-navigation-drawer
    v-show="routeLayout !== 'public' && (navType !== 'admin' || adminAuthed)"
    v-model="drawer"
    :rail="rail"
    permanent
    :width="200"
    class="nav-drawer"
  >
    <!-- Drawer 头部 -->
    <v-list-item
      :prepend-icon="rail ? headerIcon : undefined"
      class="nav-header"
      :class="rail ? 'justify-center text-center' : 'px-4'"
    >
      <template v-if="!rail">
        <v-icon color="primary" class="mr-2">{{ headerIcon }}</v-icon>
        <div>
          <div class="text-subtitle-1 font-weight-bold text-primary">{{ headerTitle }}</div>
          <div v-if="navType === 'main'" class="text-caption text-medium-emphasis">宿舍水电查询</div>
        </div>
      </template>
    </v-list-item>

    <v-divider class="my-2" />

    <!-- 用户信息（主页、非 rail 模式显示）-->
    <v-list-item
      v-if="!rail && isLoggedIn && navType === 'main'"
      :prepend-avatar="avatarUrl"
      :title="displayName"
      :subtitle="userInfo?.username || '用户'"
      class="user-info-item mx-2 my-1"
      rounded="lg"
    />

    <!-- 导航列表 -->
    <v-list density="comfortable" nav class="px-2 mt-1">
      <v-list-item
        v-for="item in navItems"
        :key="item.to"
        :to="item.to"
        :prepend-icon="item.icon"
        :title="item.title"
        rounded="lg"
        class="nav-item mb-1"
        color="primary"
      >
        <template v-if="!rail" #append>
          <v-chip
            v-if="item.badge"
            :color="item.badgeColor || 'primary'"
            size="x-small"
            variant="tonal"
          >
            {{ item.badge }}
          </v-chip>
        </template>
      </v-list-item>
    </v-list>

    <v-spacer />

    <!-- Drawer 底部 -->
    <v-list density="comfortable" nav class="px-2 py-2" />

    <!-- 收起/展开按钮 -->
    <template #append>
      <div class="rail-toggle-wrapper" :class="{ 'rail-mode': rail }">
        <v-btn
          variant="flat"
          size="small"
          class="rail-toggle-btn"
          @click="rail = !rail"
        >
          <v-icon start size="16">
            {{ rail ? 'mdi-chevron-right' : 'mdi-chevron-left' }}
          </v-icon>
          {{ rail ? '展开' : '收起' }}
        </v-btn>
      </div>
    </template>
  </v-navigation-drawer>

  <!-- ====== 顶部应用栏 ====== -->
  <v-app-bar
    v-show="routeLayout !== 'public' && (navType !== 'admin' || adminAuthed)"
    elevation="0"
    class="top-app-bar px-4"
    :class="vuetifyTheme.global.name.value === 'dark' ? 'top-app-bar-dark' : 'top-app-bar-light'"
  >
    <!-- 移动端菜单按钮（rail=true 时显示）-->
    <v-btn
      v-if="rail"
      icon="mdi-menu"
      variant="text"
      @click="drawer = !drawer"
    />

    <!-- 主页：返回按钮 -->
    <v-btn
      v-if="navType === 'main' && showBackBtn"
      icon="mdi-arrow-left"
      variant="text"
      @click="goBack"
    />

    <!-- 主页：页面标题 -->
    <v-app-bar-title v-if="navType === 'main'" class="text-body-1 font-weight-medium">
      {{ pageTitle }}
    </v-app-bar-title>

    <v-spacer />

    <!-- 主页右侧：刷新 + 主题切换 + 退出登录 -->
    <template v-if="navType === 'main' && isLoggedIn">
      <v-btn
        icon="mdi-refresh"
        variant="text"
        size="small"
        title="刷新数据"
        @click="emitter.emit('refresh')"
      />
      <v-btn
        :icon="themeIcons[themeMode]"
        variant="text"
        size="small"
        :title="themeLabels[themeMode] + '（点击切换）'"
        @click="cycleTheme"
      />
      <v-btn
        icon="mdi-logout"
        variant="text"
        size="small"
        title="退出登录"
        @click="logout"
      />
    </template>

    <!-- 管理后台右侧：主题切换 + 返回主页 + 鉴权状态 -->
    <template v-if="navType === 'admin'">
      <v-btn
        :icon="themeIcons[themeMode]"
        variant="text"
        size="small"
        :title="themeLabels[themeMode] + '（点击切换）'"
        @click="cycleTheme"
      />
      <v-btn
        variant="text"
        size="small"
        title="返回主页"
        @click="router.push('/dashboard')"
      >
        <v-icon start size="16">mdi-home-outline</v-icon>
        返回主页
      </v-btn>
      <v-chip
        :color="adminAuthed ? 'success' : 'grey'"
        variant="tonal"
        size="small"
      >
        <v-icon start size="14">{{ adminAuthed ? 'mdi-lock-open' : 'mdi-lock' }}</v-icon>
        {{ adminAuthed ? '已鉴权' : '未鉴权' }}
      </v-chip>
    </template>
  </v-app-bar>

  <!-- ====== 主内容区 ====== -->
  <v-main class="main-content">
    <v-container fluid class="pa-4 pa-md-6">
      <router-view v-slot="{ Component, route }">
        <transition name="page-fade">
          <component :is="Component" :key="route.path" />
        </transition>
      </router-view>
    </v-container>
  </v-main>

  <!-- ====== 全局 Snackbar ====== -->
  <v-snackbar
    v-model="snackbar.show"
    :color="snackbar.color"
    :timeout="3000"
    location="top"
    rounded="lg"
    elevation="4"
  >
    <div class="d-flex align-center">
      <v-icon class="mr-2" size="18">{{ snackbarIcon }}</v-icon>
      {{ snackbar.text }}
    </div>
    <template #actions>
      <v-btn variant="text" size="small" @click="snackbar.show = false">关闭</v-btn>
    </template>
  </v-snackbar>
  </v-app>
</template>

<script setup>
import { ref, computed, provide, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useTheme } from 'vuetify'
import emitter from '@/utils/eventBus.js'

const router  = useRouter()
const route   = useRoute()
const vuetifyTheme = useTheme()

// ---- 路由元信息（layout: public/default, nav: main/admin）----
const routeLayout = computed(() => route.meta?.layout || 'public')  // 安全默认值：未解析时隐藏导航
const navType     = computed(() => route.meta?.nav     || '')

// ---- 侧边栏 & 顶栏内容（根据 nav 类型切换）----
const headerIcon  = computed(() => navType.value === 'admin' ? 'mdi-shield-crown' : 'mdi-lightning-bolt')
const headerTitle = computed(() => navType.value === 'admin' ? '管理后台' : 'ElectricQuery')

const navItems = computed(() => {
  if (navType.value === 'admin') {
    return [
      { to: '/admin/sync',  icon: 'mdi-sync',          title: '数据同步' },
      { to: '/admin/users', icon: 'mdi-account-group', title: '用户管理' },
    ]
  }
  return [
    { to: '/dashboard', icon: 'mdi-view-dashboard', title: '仪表盘' },
    { to: '/history',   icon: 'mdi-history',         title: '历史记录' },
    { to: '/me',       icon: 'mdi-account-circle',  title: '我' },
  ]
})

// ---- 导航 Drawer ----
const drawer = ref(true)
const rail   = ref(false)

// ---- 全局状态 ----
const userInfo   = reactive(JSON.parse(localStorage.getItem('eq_user') || 'null') || {})
const isLoggedIn = computed(() => !!localStorage.getItem('eq_token'))

// ---- 主题三档切换 ----
const themeModes  = ['light', 'dark', 'system']
const themeIcons  = { light: 'mdi-weather-sunny', dark: 'mdi-weather-night', system: 'mdi-brightness-auto' }
const themeLabels = { light: '浅色模式', dark: '深色模式', system: '跟随系统' }
const themeMode   = ref(localStorage.getItem('eq_theme') || 'system')

const applyTheme = () => {
  if (themeMode.value === 'system') {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    vuetifyTheme.global.name.value = prefersDark ? 'dark' : 'light'
  } else {
    vuetifyTheme.global.name.value = themeMode.value
  }
}
applyTheme()
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
  if (themeMode.value === 'system') applyTheme()
})
const cycleTheme = () => {
  const idx = themeModes.indexOf(themeMode.value)
  themeMode.value = themeModes[(idx + 1) % themeModes.length]
  localStorage.setItem('eq_theme', themeMode.value)
  applyTheme()
}

// ---- 用户信息 ----
const displayName = computed(() => userInfo?.name || userInfo?.student_id || '用户')
const avatarUrl   = computed(() => {
  const char = (displayName.value || 'U')[0].toUpperCase()
  return `https://ui-avatars.com/api/?name=${char}&background=1565C0&color=fff&bold=true`
})

// ---- 页面标题 & 返回按钮 ----
const pageTitles = {
  '/dashboard': '仪表盘',
  '/me':       '个人中心',
  '/history':  '历史记录',
  '/login':    '登录',
  '/register': '注册账号',
}
const pageTitle  = computed(() => pageTitles[route.path] || 'ElectricQuery')
const showBackBtn = computed(() => false)
const goBack = () => router.back()

// ---- 退出登录 ----
const logout = () => {
  localStorage.removeItem('eq_token')
  localStorage.removeItem('eq_user')
  Object.keys(userInfo).forEach(k => delete userInfo[k])
  router.push('/login')
  notify('已退出登录', 'info')
}

// ---- 管理后台鉴权状态 ----
// 不再仅检查 localStorage 是否存在（可被绕过），改为调用后端接口真实验证
const adminAuthed = ref(false)

const validateAdminToken = async () => {
  const stored = localStorage.getItem('eq_admin_token')
  if (!stored) { adminAuthed.value = false; return }
  try {
    await adminAPI.getSyncStatus() // 需要 JWT + AdminToken，后端双重验证
    adminAuthed.value = true
  } catch {
    adminAuthed.value = false
    localStorage.removeItem('eq_admin_token')
  }
}

// 组件挂载时立即验证一次（页面刷新后仍可维持有效会话）
validateAdminToken()

// ---- 全局 Snackbar ----
const snackbar = reactive({ show: false, text: '', color: 'success' })
const snackbarIcon = computed(() => {
  const map = { success: 'mdi-check-circle', error: 'mdi-alert-circle', warning: 'mdi-alert', info: 'mdi-information' }
  return map[snackbar.color] || 'mdi-check-circle'
})
const notify = (text, color = 'success') => {
  snackbar.text  = text
  snackbar.color = color
  snackbar.show  = true
}

// provide 给所有子页面
provide('notify',      notify)
provide('userInfo',   userInfo)
provide('adminAuthed', adminAuthed)
</script>

<style>
/* ====== 侧边栏 ====== */
.nav-drawer {
  background: rgb(from var(--v-theme-surface) r g b / 0.5) !important;
  backdrop-filter: blur(8px);
  position: fixed;
  top: 0;
  left: 0;
  height: 100vh;
}
.nav-header {
  min-height: 64px;
}

/* ====== 顶栏 ====== */
.top-app-bar {
  background: rgb(from var(--v-theme-surface) r g b / 0.8) !important;
  backdrop-filter: blur(8px);
}
.top-app-bar-light {
  border-bottom: 1px solid rgba(0, 0, 0, 0.08) !important;
}
.top-app-bar-dark {
  border-bottom: 1px solid rgba(255, 255, 255, 0.08) !important;
}

/* ====== 导航项 ====== */
.nav-item {
  transition: background 0.15s ease;
}
.nav-item:hover {
  background: rgb(from var(--v-theme-primary) r g b / 0.08) !important;
}

/* ====== 用户信息 hover ====== */
.user-info-item:hover {
  background: rgb(from var(--v-theme-primary) r g b / 0.06) !important;
}

/* ====== 收起按钮 ====== */
.rail-toggle-wrapper {
  padding: 8px 12px 12px;
}
.rail-toggle-wrapper.rail-mode {
  display: flex;
  justify-content: center;
  padding: 8px 0 12px;
}
.rail-toggle-btn {
  width: 100%;
  background: rgb(from var(--v-theme-surface-variant) r g b / 0.6) !important;
  border-radius: 8px;
}

/* ====== 页面切换动画 ====== */
.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.15s ease;
}
.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}

/* ====== 公开路由（登录/注册/管理后台）：隐藏导航，内容铺满 ====== */
/* 侧边栏 */
.public-route .v-navigation-drawer {
  display: none !important;
}
/* 顶栏 */
.public-route .v-app-bar {
  display: none !important;
}
/* 内容区铺满 */
.public-route .v-main {
  padding-left: 0 !important;
  padding-top: 0 !important;
}
.public-route.v-app .v-main .v-container {
  min-height: 100vh;
  max-width: 100% !important;
}

/* ====== 管理后台未鉴权：隐藏导航栏 ====== */
/* 侧边栏 */
.admin-unauthed .v-navigation-drawer {
  display: none !important;
}
/* 顶栏 */
.admin-unauthed .v-app-bar {
  display: none !important;
}
</style>
