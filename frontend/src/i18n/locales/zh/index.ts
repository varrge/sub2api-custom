import activityLeaderboard from './activityLeaderboard'
import groupBuy from './groupBuy'
import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import imageGeneration from './imageGeneration'
import supportTickets from './supportTickets'
import modelKeyAccess from './modelKeyAccess'
import admin from './admin'
import misc from './misc'

export default {
  ...activityLeaderboard,
  ...groupBuy,
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  ...imageGeneration,
  ...supportTickets,
  ...modelKeyAccess,
  admin,
  ...misc,
}
