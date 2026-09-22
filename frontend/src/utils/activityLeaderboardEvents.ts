import { ref } from 'vue'

/**
 * Bumped when an admin saves the activity leaderboard settings, so the header
 * entry refetches the public config instead of waiting for the next poll.
 * It only carries a version counter — never user-specific leaderboard data.
 */
export const activityLeaderboardConfigVersion = ref(0)

export function notifyActivityLeaderboardConfigSaved(): void {
  activityLeaderboardConfigVersion.value += 1
}
