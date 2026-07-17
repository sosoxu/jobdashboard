import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'dashboard',
      component: () => import('@/views/Dashboard.vue'),
    },
    {
      path: '/jobs',
      name: 'jobs',
      component: () => import('@/views/JobList.vue'),
    },
    {
      path: '/jobs/:jobName/logs',
      name: 'logs',
      component: () => import('@/views/LogView.vue'),
      props: true,
    },
  ],
})

export default router
