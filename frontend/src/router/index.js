import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    redirect: '/dashboard'
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../pages/auth/LoginPage.vue'),
    meta: { layout: 'public' }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('../pages/auth/RegisterPage.vue'),
    meta: { layout: 'public' }
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('../pages/main/DashboardPage.vue'),
    meta: { layout: 'default', nav: 'main' }
  },
  {
    path: '/me',
    name: 'Me',
    component: () => import('../pages/main/MePage.vue'),
    meta: { layout: 'default', nav: 'main' }
  },
  {
    path: '/admin',
    redirect: '/admin/sync',
  },
  {
    path: '/admin/sync',
    name: 'AdminSync',
    component: () => import('../pages/admin/SyncPage.vue'),
    meta: { layout: 'default', nav: 'admin' }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('../pages/admin/UsersPage.vue'),
    meta: { layout: 'default', nav: 'admin' }
  },
  {
    path: '/profile',
    redirect: '/me'
  },
  {
    path: '/channels',
    redirect: '/me'
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫：未登录跳转到登录页
router.beforeEach((to) => {
  const token = localStorage.getItem('eq_token')
  const isPublic = to.meta?.layout === 'public'
  if (!isPublic && !token) {
    return { name: 'Login' }
  }
  if (isPublic && token && (to.name === 'Login' || to.name === 'Register')) {
    return { name: 'Dashboard' }
  }
})

export default router
