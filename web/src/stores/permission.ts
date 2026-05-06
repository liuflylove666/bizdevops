import { defineStore } from 'pinia'
import { computed } from 'vue'
import { useUserStore, type UserInfo } from './user'

/**
 * 权限 Store。基于 useUserStore 派生当前用户的角色与权限集合。
 *
 * 权限来源优先级：
 *   1. user.permissions（直接给定的权限数组）
 *   2. user.roles / user.role 根据内置映射推导
 *
 * 使用：
 *   const perm = usePermissionStore()
 *   perm.hasPermission(['system:manage'])
 *   perm.isAdmin
 */

export type RoleFocus = 'delivery' | 'operations' | 'platform'

const WILDCARD = '*'

const rolePermissionMap: Record<string, string[]> = {
  developer: [
    'pipeline:view', 'pipeline:create', 'pipeline:edit',
    'application:view', 'application:deploy',
    'k8s:view', 'logs:view'
  ],
  operator: [
    'pipeline:view', 'application:view', 'application:deploy',
    'k8s:view', 'k8s:manage', 'healthcheck:view',
    'logs:view', 'alert:view', 'cost:view'
  ],
  viewer: [
    'pipeline:view', 'application:view',
    'k8s:view', 'logs:view', 'alert:view'
  ]
}

const normalizeRoles = (user: UserInfo | null): string[] => {
  if (!user) return []
  if (Array.isArray(user.roles) && user.roles.length > 0) return user.roles
  if (typeof user.role === 'string' && user.role) return [user.role]
  return []
}

const computePermissions = (user: UserInfo | null): string[] => {
  if (!user) return []
  if (Array.isArray(user.permissions) && user.permissions.length > 0) {
    return user.permissions
  }
  const roles = normalizeRoles(user)
  if (roles.length === 0) return []
  if (roles.includes('admin') || roles.includes('administrator') || roles.includes('super_admin')) {
    return [WILDCARD]
  }
  const set = new Set<string>()
  roles.forEach(r => (rolePermissionMap[r] || []).forEach(p => set.add(p)))
  return Array.from(set)
}

const resolveRoleGroup = (roles: string[]): RoleFocus => {
  if (roles.includes('admin') || roles.includes('administrator') || roles.includes('super_admin')) return 'platform'
  if (roles.includes('operator')) return 'operations'
  return 'delivery'
}

export const usePermissionStore = defineStore('permission', () => {
  const userStore = useUserStore()

  const roles = computed(() => normalizeRoles(userStore.userInfo))
  const permissions = computed(() => computePermissions(userStore.userInfo))
  const roleGroup = computed<RoleFocus>(() => resolveRoleGroup(roles.value))
  const isAdmin = computed(() => permissions.value.includes(WILDCARD))

  /**
   * 检查是否拥有给定权限。
   *
   * - 未传或传空数组：允许（向后兼容，未声明权限的菜单/路由默认开放）
   * - 用户无权限记录：允许（向后兼容，未初始化权限的环境默认开放）
   * - 通配符 `*`：允许
   * - 支持 `resource:*` 前缀匹配
   */
  const hasPermission = (required?: string | string[]): boolean => {
    const list = required === undefined ? [] : Array.isArray(required) ? required : [required]
    if (list.length === 0) return true

    const userPerms = permissions.value
    if (userPerms.length === 0) return true
    if (userPerms.includes(WILDCARD)) return true

    return list.some(req => {
      if (req.endsWith(':*')) {
        const prefix = req.slice(0, -2)
        return userPerms.some(p => p.startsWith(prefix + ':'))
      }
      return userPerms.includes(req)
    })
  }

  return { roles, permissions, roleGroup, isAdmin, hasPermission }
})
