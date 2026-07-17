import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as apiLogin, register as apiRegister, getMe } from '@/api/auth'
import { TOKEN_KEY } from '@/api/http'

export const useUserStore = defineStore('user', () => {
  const token = ref<string | null>(localStorage.getItem(TOKEN_KEY))
  const username = ref<string | null>(null)

  const isLoggedIn = computed(() => !!token.value)

  function setSession(t: string, u: string) {
    token.value = t
    username.value = u
    localStorage.setItem(TOKEN_KEY, t)
  }

  function clearSession() {
    token.value = null
    username.value = null
    localStorage.removeItem(TOKEN_KEY)
  }

  async function login(name: string, password: string) {
    const r = await apiLogin(name, password)
    setSession(r.token, r.user.username)
  }

  async function register(name: string, password: string) {
    const r = await apiRegister(name, password)
    setSession(r.token, r.user.username)
  }

  async function fetchMe() {
    if (!token.value) return
    try {
      const u = await getMe()
      username.value = u.username
    } catch (e) {
      clearSession()
      throw e
    }
  }

  function logout() {
    clearSession()
  }

  return {
    token,
    username,
    isLoggedIn,
    login,
    register,
    logout,
    fetchMe,
  }
})
