import { defineStore } from 'pinia'
import { ref } from 'vue'
import client, { authClient } from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const username = ref('')
  const isLoggedIn = ref(!!token.value)

  if (token.value) {
    client.defaults.headers.common['Authorization'] = `Bearer ${token.value}`
  }

  async function login(user: string, pass: string) {
    try {
      const { data } = await authClient.post('/auth/login', { username: user, password: pass })
      token.value = data.access_token
      username.value = user
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
    } catch (err) {
      console.error('[auth] fetchMe failed:', err)
    }
  }

  function logout() {
    token.value = ''
    username.value = ''
    localStorage.removeItem('token')
    delete client.defaults.headers.common['Authorization']
    isLoggedIn.value = false
  }

  return { token, username, isLoggedIn, login, fetchMe, logout }
})
