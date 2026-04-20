<template>
  <v-app :theme="theme">
    <!-- ====== 侧边导航 Drawer (MD3 NavigationRail on desktop) ====== -->
    <v-navigation-drawer
      v-model="drawer"
      :rail="rail"
      permanent
      :width="240"
      class="nav-drawer"
    >
      <!-- Drawer 头部 -->
      <v-list-item
        :prepend-icon="rail ? 'mdi-lightning-bolt' : undefined"
        class="nav-header"
        :class="rail ? 'justify-center' : 'px-4'"
      >
        <template v-if="!rail">
          <v-icon color="primary" class="mr-2">mdi-lightning-bolt</v-icon>
          <div>
            <div class="text-subtitle-1 font-weight-bold text-primary">ElectricQuery</div>
            <div class="text-caption text-medium-emphasis">宿舍水电查询</div>
          </div>
        </template>
      </v-list-item>

      <v-divider class="my-2" />

      <!-- 用户信息（仅在非 rail 模式显示）-->
      <v-list-item
        v-if="!rail && isLoggedIn"
        :prepend-avatar="avatarUrl"
        :title="displayName"
        :subtitle="userInfo?.student_id || '未绑定学号'"
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
      <v-divider />
      <v-list density="comfortable" nav class="px-2 py-2">
        <!-- 主题切换 -->
        <v-list-item
          rounded="lg"
          class="nav-item mb-1"
          :prepend-icon="theme === 'light' ? 'mdi-weather-sunny' : 'mdi-weather-night'"
          :title="theme === 'light' ? '浅色模式' : '深色模式'"
          @click="toggleTheme"
        />
        <!-- 退出登录 -->
        <v-list-item
          v-if="isLoggedIn"
          rounded="lg"
          class="nav-item"
          prepend-icon="mdi-logout"
          title="退出登录"
          @click="logout"
        />
      </v-list>

      <!-- Rail 模式展开按钮 -->
      <template #append>
        <v-btn
          :icon="rail ? 'mdi-chevron-right' : 'mdi-chevron-left'"
          variant="text"
          size="small"
          class="rail-toggle-btn"
          @click="rail = !rail"
        />
      </template>
    </v-navigation-drawer>

    <!-- ====== 顶部应用栏 (MD3 TopAppBar) ====== -->
    <v-app-bar
      elevation="0"
      class="top-app-bar px-4"
      :style="'border-bottom: 1px solid rgba(0,0,0,0.08);'"
    >
      <!-- 移动端菜单按钮（rail=true 时显示）-->
      <v-btn
        v-if="rail"
        icon="mdi-menu"
        variant="text"
        @click="drawer = !drawer"
      />

      <!-- 返回按钮（子页面显示）-->
      <v-btn
        v-if="showBackBtn"
        icon="mdi-arrow-left"
        variant="text"
        class="mr-1"
        @click="goBack"
      />

      <!-- 页面标题 -->
      <v-app-bar-title class="text-body-1 font-weight-medium">
        {{ pageTitle }}
      </v-app-bar-title>

      <v-spacer />

      <!-- 登录用户信息（非 rail 桌面模式）-->
      <template v-if="!rail && isLoggedIn" #append>
        <v-chip
          class="mr-2"
          color="primary-container"
          text-color="on-primary-container"
          variant="tonal"
          size="small"
        >
          <v-icon start size="14">mdi-account</v-icon>
          {{ userInfo?.student_id || '未绑定' }}
        </v-chip>
      </template>
    </v-app-bar>

    <!-- ====== 主内容区 ====== -->
    <v-main class="main-content">
      <v-container fluid class="pa-4 pa-md-6">
        <router-view v-slot="{ Component, route }">
          <transition name="page-fade" mode="out-in">
            <component :is="Component" :key="route.path" />
          </transition>
        </router-view>
      </v-container>
    </v-main>

    <!-- ====== 全局 Snackbar 提示 ====== -->
    <v-snackbar
      v-model="snackbar.show"
      :color="snackbar.color"
      :timeout="3000"
      location="top"
      rounded="lg"
      elevation="4"
    >
      <div class="d-flex align-center">
        <v-icon class="mr-2" size="18">
          {{ snackbarIcon }}
        </v-icon>
        {{ snackbar.text }}
      </div>
      <template #actions>
        <v-btn variant="text" size="small" @click="snackbar.show = false">
          关闭
        </v-btn>
      </template>
    </v-snackbar>
  </v-app>
</template>

<script setup>
import { ref, computed, provide, reactive, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'

const router  = useRouter()
const route   = useRoute()

// ---- 全局状态 ----
const userInfo   = reactive(JSON.parse(localStorage.getItem('eq_user') || 'null') || {})
const isLoggedIn = computed(() => !!localStorage.getItem('eq_token'))

// ---- 主题 ----
const theme  = ref('light')
const toggleTheme = () => {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
}

// ---- 导航 Drawer ----
const drawer = ref(true)
const rail   = ref(false)  // 桌面端默认展开侧边栏

// ---- 导航项 ----
const navItems = computed(() => {
  const items = [
    { to: '/dashboard', icon: 'mdi-view-dashboard', title: '仪表盘' },
    { to: '/profile',   icon: 'mdi-account-edit',   title: '个人信息' },
    { to: '/channels', icon: 'mdi-bell-cog',        title: '通知渠道' },
  ]
  // 未绑定学号显示提示
  if (!userInfo?.student_id) {
    items[1] = { ...items[1], badge: '必填', badgeColor: 'error' }
  }
  return items
})

// ---- 用户信息 ----
const displayName = computed(() => {
  return userInfo?.name || userInfo?.student_id || '用户'
})
const avatarUrl = computed(() => {
  // 生成首字母头像颜色
  const char = (displayName.value || 'U')[0].toUpperCase()
  return `https://ui-avatars.com/api/?name=${char}&background=1565C0&color=fff&bold=true`
})

// ---- 页面标题 & 返回按钮 ----
const pageTitles = {
  '/dashboard': '仪表盘',
  '/profile':   '个人信息',
  '/channels':  '通知渠道',
  '/login':     '登录',
  '/register':  '注册账号',
}
const pageTitle  = computed(() => pageTitles[route.path] || 'ElectricQuery')
const showBackBtn = computed(() =>
  ['/profile', '/channels'].includes(route.path)
)
const goBack = () => router.back()

// ---- 退出登录 ----
const logout = () => {
  localStorage.removeItem('eq_token')
  localStorage.removeItem('eq_user')
  Object.keys(userInfo).forEach(k => delete userInfo[k])
  router.push('/login')
  notify('已退出登录', 'info')
}

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
provide('notify', notify)
provide('userInfo', userInfo)
</script>

<style>
/* ====== MD3 Surface Tint 叠加效果 ====== */
.nav-drawer {
  background: rgb(from var(--v-theme-surface) r g b / 0.5) !important;
  backdrop-filter: blur(8px);
}

.top-app-bar {
  background: rgb(from var(--v-theme-surface) r g b / 0.8) !important;
  backdrop-filter: blur(8px);
}

.nav-item {
  transition: background 0.15s ease;
}
.nav-item:hover {
  background: rgb(from var(--v-theme-primary) r g b / 0.08) !important;
}

/* MD3 Ripple 使用 Vuetify 内置，无需额外配置 */

/* 页面切换动画 */
.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.page-fade-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.page-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

.rail-toggle-btn {
  position: absolute;
  right: -12px;
  top: 50%;
  transform: translateY(-50%);
  background: var(--v-theme-surface) !important;
  border: 1px solid rgba(0,0,0,0.1) !important;
  z-index: 10;
}

/* 用户信息项 hover 效果 */
.user-info-item:hover {
  background: rgb(from var(--v-theme-primary) r g b / 0.06) !important;
}
</style>
