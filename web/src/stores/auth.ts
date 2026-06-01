import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authApi } from '../api/client'

interface User {
  id: number
  username: string
  role: string
}

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(null)

  function init() {
    const savedToken = localStorage.getItem('token')
    const savedUser = localStorage.getItem('user')
    if (savedToken && savedUser) {
      token.value = savedToken
      try {
        user.value = JSON.parse(savedUser)
      } catch {
        logout()
      }
    }
  }

  async function login(username: string, password: string) {
    const res = await authApi.login(username, password)
    token.value = res.data.token
    user.value = res.data.user
    localStorage.setItem('token', res.data.token)
    localStorage.setItem('user', JSON.stringify(res.data.user))
  }

  function logout() {
    user.value = null
    token.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  async function changePassword(oldPassword: string, newPassword: string) {
    await authApi.changePassword(oldPassword, newPassword)
  }

  init()

  return { user, token, login, logout, changePassword }
})
