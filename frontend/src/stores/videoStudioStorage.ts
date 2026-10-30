import type { VideoStudioSession } from './videoStudio'

export interface SavedVideoStudio {
  sessions: VideoStudioSession[]
  activeId: string | null
  revision?: number
}

async function database(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('sub2api-video-studio', 1)
    request.onupgradeneeded = () => request.result.createObjectStore('workspaces')
    request.onerror = () => reject(request.error)
    request.onblocked = () => reject(new Error('Video studio storage is blocked'))
    request.onsuccess = () => resolve(request.result)
  })
}

export async function loadVideoStudio(userId: number): Promise<SavedVideoStudio | null> {
  const db = await database()
  try {
    return await new Promise((resolve, reject) => {
      const request = db.transaction('workspaces').objectStore('workspaces').get(userId)
      request.onsuccess = () => resolve(request.result ?? null)
      request.onerror = () => reject(request.error)
    })
  } finally {
    db.close()
  }
}

export async function saveVideoStudio(userId: number, state: SavedVideoStudio): Promise<number> {
  const db = await database()
  try {
    return await new Promise<number>((resolve, reject) => {
      const transaction = db.transaction('workspaces', 'readwrite')
      const revision = (state.revision ?? 0) + 1
      transaction.oncomplete = () => resolve(revision)
      transaction.onerror = () => reject(transaction.error)
      transaction.onabort = () => reject(transaction.error)
      const store = transaction.objectStore('workspaces')
      const current = store.get(userId)
      current.onsuccess = () => {
        if ((current.result?.revision ?? 0) !== (state.revision ?? 0)) {
          reject(new Error('video-studio-conflict'))
          transaction.abort()
          return
        }
        store.put({ ...state, revision }, userId)
      }
    })
  } finally {
    db.close()
  }
}
