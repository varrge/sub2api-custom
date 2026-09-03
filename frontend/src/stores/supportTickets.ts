import { defineStore } from 'pinia'
import { ref } from 'vue'
import { supportTicketsAdminAPI, supportTicketsUserAPI } from '@/features/support-tickets/api'

export const useSupportTicketStore = defineStore('supportTickets', () => {
  const userUnreadCount = ref(0)
  const adminUnreadCount = ref(0)
  const userUnreadLoaded = ref(false)
  const adminUnreadLoaded = ref(false)

  async function refreshUserUnread(): Promise<number> {
    const count = await supportTicketsUserAPI.unreadCount()
    userUnreadCount.value = count
    userUnreadLoaded.value = true
    return count
  }

  async function refreshAdminUnread(): Promise<number> {
    const count = await supportTicketsAdminAPI.unreadCount()
    adminUnreadCount.value = count
    adminUnreadLoaded.value = true
    return count
  }

  function reset(): void {
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
    reset,
  }
})
