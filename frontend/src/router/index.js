import { createRouter, createWebHistory } from 'vue-router'
import Bind from '../pages/Bind.vue'

const routes = [
  { path: '/', redirect: '/bind' },
  { path: '/bind', name: 'Bind', component: Bind }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router
