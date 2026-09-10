# 拼团月卡实施计划与接口约定

用户已授权开始实现。业务来源为 design.md、ui-design.md、verification.md。本文件是实现约定与进度记录，出现冲突以已确认业务规则为准。

## 工作分工

1. 核心存储与拼团：新增 `backend/internal/monthcard/` 产品、团、卡存储与生命周期以及 SQL 迁移，业务与测试不依赖 service 包。
2. 订阅准入与结算：新增该包的 billing 文件、现有订阅/鉴权/计费接入，支持旧订阅候选及历史周期结算。
3. 用户与管理前端：独占 frontend 目录，完成大厅、购买页签、我的订阅及后台商品、团管理。
4. 主代理：支付订单/履约/退款、HTTP handler、依赖装配、集成验证、审查与修复。

边界：不改现有旧订阅表的唯一约束，月卡独立存表；不覆盖开始前 pricing_service 及相关测试的用户改动。保持当前 custom/0.2.0 工作区，不发布、不操作远程。

## 共享 Go 接口（包 monthcard）

采用独立 SQL Store，`NewStore(db *sql.DB) *Store`。JSON 使用 snake_case，金额使用 float64 对外传输、DB NUMERIC(20,8) 与 decimal 计算。新表命名前缀 `month_card_`。无商品或卡的现有用户继续旧路径；新模式由存在月卡或该用户分组有月卡历史触发，旧模式行为保持兼容。

核心所有者提供：

- `Tier {Members int json:members, QuotaUSD float64 json:quota_usd}`。
- `Product {ID,GroupID int64; GroupName,Platform,Name,Description string; PriceCNY,BaseQuotaUSD float64; Tiers []Tier; MaxMembers,RecruitmentHours int; ForSale bool; SortOrder int}`；JSON 如 `price_cny`, `base_quota_usd`, `recruitment_hours`。
- `Team {ID int64; Code string; ProductID int64; Product Product; MemberCount int; CurrentQuotaUSD,NextQuotaUSD float64; NextMembers int; StartsAt,ClosesAt time.Time; Status string; Joined bool}`；status recruiting/full/closed。
- `Card {ID,UserID,GroupID,OrderID int64; Code,GroupName,Platform,ProductName,TeamCode,Status string; TeamID *int64; TotalQuotaUSD,TotalUsedUSD,WeeklyQuotaUSD,WeeklyUsedUSD float64; StartsAt,ExpiresAt,WeeklyWindowStart,WeeklyWindowEnd time.Time; Priority int}`；JSON字段对应 snake_case。status active/expired/revoked。
- `Purchase {Mode string; Product Product; TeamID *int64; TeamCode string}`；Mode solo/create/join。
- `ErrCannotJoin` 支付后无法参团应走原路退款，`ErrNotFound`、`ErrInvalid` 可识别错误。
- `ListProducts(ctx, admin bool) ([]Product,error)`, `SaveProduct(ctx, *Product) error`（ID=0创建）, `GetProduct(ctx,id) (*Product,error)`。
- `ListTeams(ctx,userID,groupID int64,admin bool) ([]Team,error)`, `GetTeam(ctx,code string,userID int64) (*Team,error)`。
- `PreparePurchase(ctx,userID,productID int64,mode,teamCode string) (*Purchase,error)` 验证用户/分组权限与商品可售；join 使用团快照。
- `Fulfill(ctx,orderID,userID int64,paidAt time.Time,purchase *Purchase) (*Card,error)`：锁订单，幂等创建；团锁保护人数上限、每用户仅一次；加额不改周期和已用量。
- `RevokeOrder(ctx,orderID int64) error`：幂等撤卡，退款成员不计后续档位，团历史额度不降。
- `ListCards(ctx,userID int64) ([]Card,error)`；admin 查询可附加具体方法。
- `SetOrder(ctx,userID,groupID int64,refs []Ref) error` 全量原子校验同用户同分组，排序包含 legacy 与 card。
- 核心迁移建立 `month_card_products`、`month_card_teams`、`month_card_cards`、`month_card_priorities`；核心必须即时向结算所有者发送准确 SQL schema。

结算所有者提供（与核心同包，不修改核心文件）：

- `Ref {Kind string json:kind; ID int64 json:id}`，kind card/legacy。
- `Candidate {Ref; WeeklyWindowStart time.Time; DailyWindowStart,MonthlyWindowStart *time.Time}`；可添加必要的限制/资格快照字段。
- `Snapshot {UserID,GroupID int64; StartedAt time.Time; Candidates []Candidate}`。
- `Store.Admit(ctx,userID,groupID int64,at time.Time) (*Snapshot,error)`：nil,nil 表示该用户分组尚无月卡历史，继续原旧路径；有新卡历史则校验欠费、收集同组新旧可用权益、按顺序快照；无可用权益返回明确错误。
- `SettleTx(ctx,tx *sql.Tx,snapshot *Snapshot,requestID string,apiKeyID int64,cost float64) (*Settlement,error)`，由现有 usage_billing 事务在幂等 claim 成功后调用；不得独立提交事务。
- `Settlement {BalanceCost float64; NewBalance *float64; Allocations []Allocation}`；分摊/余额合计精确等于已量化实际费用。
- 历史周期及分摊表由结算所有者增加单独迁移，编号与核心协调。
- `Store.ListAllocations(ctx,userID int64) ([]Allocation,error)` 供用户明细与管理员查询（限制最近100条或分页）。
- 在 `service.UserSubscription` 增加 `MonthCardSnapshot *monthcard.Snapshot`，用作跨现有 handler 传递的准入快照。新卡虚拟订阅不得把卡 ID 写入现有 usage_logs 的旧订阅外键。
- SubscriptionService/BillingCacheService 通过 setter 注入 `*monthcard.Store`，签名保持现有测试兼容；主代理负责在 Wire provider 装配。结算所有者负责 service、repository、middleware 其他接入及测试。

## HTTP 与前端约定（基础路径 /api/v1）

- GET `/group-buy/products` → Product[]。
- GET `/group-buy/teams?group_id=` → Team[]（正在招募）；GET `/group-buy/teams/:code` → Team。
- GET `/group-buy/cards` → Card[]（当前用户）。
- GET `/group-buy/order` → `[{group_id,items:[{kind,id}]}]`，供统一展示与编辑顺序。
- PUT `/group-buy/order` body `{group_id,items:[{kind:'card'|'legacy',id}]}` → success。
- GET `/group-buy/allocations` → Allocation[]。
- POST `/payment/orders` 沿用原请求并支持 `{order_type:'month_card',product_id,mode:'solo'|'create'|'join',team_code?,payment_type,...}`，响应沿用现有支付响应；价格以后端为准。
- GET `/admin/group-buy/products`（全部），POST 同路径，PUT `/admin/group-buy/products/:id`，body Product 可配置字段。
- GET `/admin/group-buy/teams`（含历史），GET `/admin/group-buy/teams/:code`。
- GET `/admin/group-buy/teams/:code/cards` → 团成员的卡与付款订单关联。
- GET `/admin/group-buy/cards?user_id=` 与 `/admin/group-buy/allocations?user_id=`。
- 退款复用现有 `/admin/payment/orders/:id/refund`，服务按 month_card 类型撤销卡；只支持全单退款（人工审核），不自动修改旧订阅期限。
- POST `/admin/payment/orders/:id/refund/confirm` body `{reference,confirmed:true}`：仅已发起整单退款的月卡可在原渠道人工核实后完成本地撤卡，保留渠道流水/说明与操作审计，不再次发起退款。

主代理负责以上 HTTP 与支付；前端可按此独立实现。所有端点使用既有 JWT/admin 鉴权和 response 包装。

## 技术取舍记录

- 新卡使用独立表，不移除旧表唯一索引。降低旧销售/兑换兼容风险；代价是聚合展示与混合结算需适配。
- 余额补扣采用实际费用结算并允许欠费，卡份额仍原子封顶，不为所有模型强制预估最大费用。
- 固定人民币售价只允许实际收款币种为 CNY 的支付实例，避免把人民币数字直接当外币金额。前端给出清晰错误；现有其它购买不变。
- 退款与在途结算通过用户/卡一致锁序串行；在途已实际发生费用不能遗失，退款后卡不可再扣的差额记余额并记录。
- 排序实现建议采用用户分组顺序表，已设置顺序保留，新卡追加；初始按到期时间和 ID。
- 结算命令在应用扣费前持久化，恢复任务重放同一请求；扣费去重、卡/余额分摊和删除待重试命令在同一事务内提交。
- 旧订阅的手动重置使用周期代次，避免同一午夜窗口重置后沿用旧用量，也避免在途旧请求消耗新授予额度。
- 中断的退款先查询原渠道结果，再仅对有稳定幂等退款号的渠道重发；无查询能力的渠道可由管理员核实原路退款后完成撤卡。
- Grok 异步视频将创建时的权益及路由归属持久化，完成查询使用原快照；WS 每轮重新准入并冻结该轮快照。
- 原批量生图使用单独余额预冻结流程，原 OpenAI live 路径目前计费为零；本次保留这两条既有路径。批量生图是否纳入月卡已作为补充范围问题提出，尚未收到用户选择，不视为已确认新需求。

## 进度

- 已完成：19组需求与页面方案、25条验收场景，以及核心存储、准入结算、支付退款、接口与前端实现。
- 已完成：真实 PostgreSQL 并发/竞态与完整迁移测试，独立代码审查及问题修复，桌面与手机浏览器页面检查。
- 本地验证结果见 [verification.md](verification.md)，细分报告见 [core-report.md](core-report.md)、[billing-report.md](billing-report.md)、[frontend-report.md](frontend-report.md)。
- 未执行发布：没有提交、推送、打标签或操作远程环境；CI、中国 staging 与真实支付渠道验收属于后续发布门槛。
