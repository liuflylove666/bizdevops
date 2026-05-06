import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

/**
 * 用户会话 Store。
 *
 * 单一真相源：当前用户身份（token + userInfo）。
 * 内部仍同步写 localStorage，保证尚未迁移的老代码（直接读 localStorage）继续工作。
 *
 * 典型使用：
 *   const userStore = useUserStore()
 *   userStore.setSession(token, user)  // 登录成功
 *   userStore.clearSession()           // 登出 / 401
 *   userStore.token / userStore.userInfo / userStore.isLoggedIn
 */

const TOKEN_KEY = 'token'
const USER_INFO_KEY = 'userInfo'

export interface UserInfo {
  id?: number
  username?: string
  email?: string
  roles?: string[]
  role?: string
  permissions?: string[]
  [k: string]: any
}

const safeParse = (raw: string | null): UserInfo | null => {
  if (!raw) return null
  try {
    return JSON.parse(raw) as UserInfo
  } catch {
    return null
  }
}

export const useUserStore = defineStore('user', () => {
  const token = ref<string>(typeof window !== 'undefined' ? localStorage.getItem(TOKEN_KEY) || '' : '')
  const userInfo = ref<UserInfo | null>(
    typeof window !== 'undefined' ? safeParse(localStorage.getItem(USER_INFO_KEY)) : null
  )

  const isLoggedIn = computed(() => Boolean(token.value))
  const username = computed(() => userInfo.value?.username || '')

  const setSession = (nextToken: string, nextUser: UserInfo | null) => {
    token.value = nextToken
    userInfo.value = nextUser
    if (typeof window === 'undefined') return
    try {
      localStorage.setItem(TOKEN_KEY, nextToken)
      if (nextUser) {
        localStorage.setItem(USER_INFO_KEY, JSON.stringify(nextUser))
      } else {
        localStorage.removeItem(USER_INFO_KEY)
      }
    } catch (e) {
      console.error('Failed to persist user session', e)
    }
  }

  const clearSession = () => {
    token.value = ''
    userInfo.value = null
    if (typeof window === 'undefined') return
    try {
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(USER_INFO_KEY)
    } catch (e) {
      console.error('Failed to clear user session', e)
    }
  }

  // 从 localStorage 同步（供外部代码在 storage 事件后强制刷新，当前未绑定监听）
  const loadFromStorage = () => {
    if (typeof window === 'undefined') return
    token.value = localStorage.getItem(TOKEN_KEY) || ''
    userInfo.value = safeParse(localStorage.getItem(USER_INFO_KEY))
  }

  return { token, userInfo, isLoggedIn, username, setSession, clearSession, loadFromStorage }
})
