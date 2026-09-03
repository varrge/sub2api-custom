import { defineStore } from 'pinia'
import { ref } from 'vue'
import { supportTicketsAdminAPI, supportTicketsUserAPI } from '@/features/support-tickets/api'

export const useSupportTicketStore = defineStore('supportTickets', () => {
  const userUnreadCount = ref(0)
  const adminUnreadCount = ref(0)
  const userUnreadLoaded = ref(false)
  const adminUnreadLoaded = ref(false)
  let sessionGeneration = 0
  let userRequestGeneration = 0
  let adminRequestGeneration = 0
  let userInitialization: Promise<number> | null = null
  let adminInitialization: Promise<number> | null = null

  async function refreshUserUnread(): Promise<number> {
    const session = sessionGeneration
    const request = ++userRequestGeneration
    const count = await supportTicketsUserAPI.unreadCount()
    if (session === sessionGeneration && request === userRequestGeneration) {
      userUnreadCount.value = count
      userUnreadLoaded.value = true
    }
    return count
  }

  async function refreshAdminUnread(): Promise<number> {
    const session = sessionGeneration
    const request = ++adminRequestGeneration
    const count = await supportTicketsAdminAPI.unreadCount()
    if (session === sessionGeneration && request === adminRequestGeneration) {
      adminUnreadCount.value = count
      adminUnreadLoaded.value = true
    }
    return count
  }

  function initializeUserUnread(): Promise<number> {
    if (userUnreadLoaded.value) return Promise.resolve(userUnreadCount.value)
    if (userInitialization) return userInitialization
    const pending = refreshUserUnread()
    userInitialization = pending
    void pending.then(() => {
      if (userInitialization === pending) userInitialization = null
    }, () => {
      if (userInitialization === pending) userInitialization = null
    })
    return pending
  }

  function initializeAdminUnread(): Promise<number> {
    if (adminUnreadLoaded.value) return Promise.resolve(adminUnreadCount.value)
    if (adminInitialization) return adminInitialization
    const pending = refreshAdminUnread()
    adminInitialization = pending
    void pending.then(() => {
      if (adminInitialization === pending) adminInitialization = null
    }, () => {
      if (adminInitialization === pending) adminInitialization = null
    })
    return pending
  }

  function reset(): void {
    sessionGeneration++
    userRequestGeneration++
    adminRequestGeneration++
    userInitialization = null
    adminInitialization = null
    userUnreadCount.value = 0
    adminUnreadCount.value = 0
    userUnreadLoaded.value = false
    adminUnreadLoaded.value = false
  }

  return {
    userUnreadCount,
    adminUnreadCount,
    userUnreadLoaded,
    adminUnreadLoaded,
    refreshUserUnread,
    refreshAdminUnread,
    initializeUserUnread,
    initializeAdminUnread,
    reset,
  }
})
