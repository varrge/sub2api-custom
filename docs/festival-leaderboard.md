# 2026 双节消费榜

顶部入口顺序：主题切换 → 双节消费榜（奖杯）→ 余额。点击打开弹窗，展示前 20 名及当前用户自己的排名；榜单仅登录后可见。

## 统计口径

- 活动时间：北京时间 2026-09-25 00:00（含）至 2026-10-08 00:00（不含）。时间在 `backend/internal/service/activity_leaderboard_service.go` 中由服务端固定，客户端查询参数无法修改。
- 按 `usage_logs.created_at` 归属活动，累加 `actual_cost > 0` 的调用记录。包含全分组、所有模型和媒体、余额、订阅及拼团月卡，保留每次调用落账时的个人/优惠倍率；不使用当前分组倍率重新计算。
- 充值和套餐购买金额不属于调用消耗。月卡分摊、余额补扣不单独重复相加；依赖已有 `(request_id, api_key_id)` 唯一约束避免请求重试重复记账。
- 存在于 `month_card_billing_pending` 的待结算请求暂不计入，恢复成功后按原调用记录时间计入。统计基于用量记录，不能替代支付流水或财务核账。
- 排除管理员与已删除账号。普通测试账号不会根据邮箱或昵称自动排除；本次未加入测试账号名单管理，发奖前需核验资格。
- 金额以 PostgreSQL NUMERIC 原精度排序；同额按最后一笔正计费时间较早者优先，再按内部用户 ID 排序。前端显示最多四位小数，完整额度可通过金额提示查看。
- 展示的是**计费额度，不是人民币实付金额**。前三名奖励仅展示“另行公布”，未写入未经确认的奖励金额。

## 接口和缓存

`GET /api/v1/activities/double-festival/leaderboard` 使用现有 JWT 认证、用户后台模式守卫和面板重查询限流。只返回匿名编号、名次和额度，不返回邮箱、用户名或其他用户 ID；编号由 JWT 密钥对活动 ID 和用户 ID 做 HMAC 后截取 48 位生成。更换 JWT 密钥将改变匿名编号。

每个进程保留一份活动快照，60 秒有效，并合并并发刷新请求；查询有 15 秒超时。Top 20 与本人排名来自同一快照，返回时复制个性化字段，HTTP 使用 `private, no-store`。未开始时不查询用量表；活动结束后继续定期重算以接纳延迟结算，页面注明奖励以核验为准。

弹窗打开后才请求，打开期间每分钟刷新，后台标签页暂停定时请求；关闭/卸载会取消请求并清理定时器。刷新失败保留上次数据并提示，首次失败提供重试。日期始终按北京时间展示，中英文、明暗主题均支持。

## 本地验证

```sh
cd backend
go test ./internal/service ./internal/repository ./internal/handler -run TestActivityLeaderboard -count=1
go test -race ./internal/service -run TestActivityLeaderboard -count=1
# 对隔离的 PostgreSQL 执行真实 SQL；测试自行创建并清理 schema。
ACTIVITY_TEST_DSN='postgres://USER@127.0.0.1:PORT/postgres?sslmode=disable' go test ./internal/repository -run TestActivityLeaderboardPostgresAccounting -count=1 -v
cd ../frontend
pnpm test:run src/components/common/__tests__/ActivityLeaderboard.spec.ts src/components/layout/__tests__/AppHeader.quickMenu.spec.ts src/i18n/__tests__/localeKeyCompleteness.spec.ts
pnpm typecheck
pnpm lint:check
pnpm build
```

无数据库迁移，不修改活动价格、公开注册、套餐、奖励发放配置或生产部署。

## 验证结果（2026-09-22）

- 前端全量：332 个测试文件、2469 项测试通过；最终布局调整后，相关 20 项测试再次通过。
- 后端 service / handler / repository 三个包测试通过；榜单 race 检测及独立 PostgreSQL 16 真实查询验证通过。
- 独立代码审查无遗留阻塞项。Wire 重新生成时发现原生成文件中有手工插件注入，现通过 `ProvidePluginManager` 保留账号目录能力；相关插件与服务装配检查通过。
- 前端类型检查、ESLint 及生产构建通过。构建仍有项目现存的大 chunk 提示。
- 浏览器使用本地模拟接口验证，不读取或写入生产数据；示例榜单仅存在于临时测试脚本中。
- 最终浏览器复测：320 / 375 / 390 / 640 / 768 / 1024 / 1440px 均无横向溢出、导航重叠或页面异常；入口顺序、弹窗关闭和焦点恢复正常。

## 中国部署候选适配

部署分支 `feat/cn-festival-leaderboard` 基于中国服务器当前 `727a8d367` 黑金主题版本，应用上述已审查改动；保留现有主题、首页和模型广场。窄屏使用可截断余额，移除旧版 359px 以下隐藏余额的规则。未合并任何分支、未创建发布标签。
