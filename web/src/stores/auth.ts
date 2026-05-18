import { defineStore } from 'pinia'
import { ref } from 'vue'
import client, { authClient } from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const username = ref('')
  const isLoggedIn = ref(!!token.value)

  // Bootstrap Authorization header from persisted token
  if (token.value) {
    client.defaults.headers.common['Authorization'] = `Bearer ${token.value}`
  }

  async function login(user: string, pass: string) {
    const { data } = await authClient.post('/auth/login', { username: user, password: pass })
    token.value = data.token
    username.value = user
    localStorage.setItem('token', data.token)
    client.defaults.headers.common['Authorization'] = `Bearer ${data.token}`
    isLoggedIn.value = true
  }

  async function fetchMe() {
    const { data } = await authClient.get('/auth/me')
    username.value = data.username || username.value
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