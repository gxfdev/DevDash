import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import client, { authClient } from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const username = ref(localStorage.getItem('auth-username') || '')
  const role = ref(localStorage.getItem('auth-role') || '')
  const mustChangePwd = ref(localStorage.getItem('auth-must-change-pwd') === 'true')
  const isLoggedIn = ref(!!token.value)

  if (token.value) {
    client.defaults.headers.common['Authorization'] = `Bearer ${token.value}`
  }

  watch(mustChangePwd, (v) => {
    localStorage.setItem('auth-must-change-pwd', String(v))
  })

  watch(username, (v) => {
    if (v) localStorage.setItem('auth-username', v)
  })

  watch(role, (v) => {
    if (v) localStorage.setItem('auth-role', v)
  })

  async function login(user: string, pass: string) {
    try {
      const { data } = await authClient.post('/auth/login', { username: user, password: pass })
      token.value = data.access_token
      username.value = user
      mustChangePwd.value = !!data.must_change_pwd
      localStorage.setItem('token', data.access_token)
      client.defaults.headers.common['Authorization'] = `Bearer ${data.access_token}`
      isLoggedIn.value = true
    } catch (err) {
      isLoggedIn.value = false
      throw err
    }
  }

  async function fetchMe() {
    try {
      const { data } = await authClient.get('/auth/me')
      username.value = data.username || username.value
      role.value = data.role || role.value
      mustChangePwd.value = !!data.must_change_pwd
    } catch (err: unknown) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (status === 401 || status === 403) {
        logout()
        window.location.href = '/login'
      }
    }
  }

  function logout() {
    token.value = ''
    username.value = ''
    role.value = ''
    mustChangePwd.value = false
    localStorage.removeItem('token')
    localStorage.removeItem('auth-username')
    localStorage.removeItem('auth-role')
    localStorage.removeItem('auth-must-change-pwd')
    delete client.defaults.headers.common['Authorization']
    isLoggedIn.value = false
  }

  return { token, username, role, mustChangePwd, isLoggedIn, login, fetchMe, logout }
})
