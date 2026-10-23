import activityLeaderboard from './activityLeaderboard'
import luckyBox from './luckyBox'
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
import enterpriseNav from './enterpriseNav'
import enterpriseAdmin from './enterpriseAdmin'
import enterprisePlaza from './enterprisePlaza'
import enterpriseExport from './enterpriseExport'
import enterprisePayment from './enterprisePayment'

export default {
  ...activityLeaderboard,
  ...luckyBox,
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
  ...enterpriseNav,
  ...enterpriseAdmin,
  ...enterprisePlaza,
  ...enterpriseExport,
  ...enterprisePayment,
}
