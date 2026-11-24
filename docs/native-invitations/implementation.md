# 原生注册邀请

本文保留原候选实现的规则与历史验证记录。本次仅将邀请码和返利接入
`custom/0.2.22`，当前合并范围、修复和验证见 [合并验证](merge-verification.md)。

需求来源：卡网 `shared/src/sub2api.js`、`api/src/server.js` 和
`docs/adr/0004-local-registration-invite-rebate-layer.md` 的邀请业务。
本实现全部运行在 Sub2API 的进程、认证体系、PostgreSQL 和前端内。
不调用卡网或远端管理员 API，不使用通用 affiliate 码代替一次性注册邀请码。

## 规则

- 复用本地 `redeem_codes.type=invitation` 与邮箱/OAuth 注册事务。
- 已登录的个人账户申请邀请码；随机码、有效期、归属和申请幂等键一起落库。
- 等级按成功的正数 balance 兑换记录累计额度（包括本地支付充值对应的兑换记录）实时计算；
  排除订阅、管理员赠送、邀请奖励及当前余额。支持管理员指定等级；
  升级差额仍按自动等级计算。
- 每级配置累计申请上限、当前未使用上限、返利比例。上限 0 表示不限；
  已过期且未使用的码不占额度，已使用的码一直计入累计额度。
- 使用注册邀请码时锁定邀请人，不受后续 affiliate 关系修改影响。
  该被邀请人不再产生通用 affiliate 返利，避免两套规则重复奖励。
- 首次成功取得正数余额产生且仅产生一条待审核记录。在线支付按订单实际支付金额、
  卡密按兑换面值计算，记录来源及币种；比例使用 0–100 百分数，
  返利按金额 × 比例 ÷ 100，十进制四舍五入至两位。
  与卡网现行规则一致，返利数值记入 Sub2API 美元余额，后台明确显示原始支付币种。
  订阅、赠送、负数调整、后续充值均不产生第二条返利。
- 首次成功到账与比例/等级快照在同一事务中保存；普通支付的内部充值码只计一次。
- 超时自动入账（`auto_approve_hours`，默认 0 即关闭）：待审核超过该小时数的返利由后台任务
  自动批准并入账，对应卡网“超过 24 小时未处理自动通过”的规则，但时长可配置。
  以下情况不自动处理，继续留在人工队列：邀请人或被邀请人已不是正常的个人账户；
  在线支付的首付订单当前状态不是 COMPLETED（例如已退款或退款中）。自动入账的审计记录
  操作人为空，列表中标注“超时自动入账”，管理员之后仍可撤销。
- 待审核可批准或拒绝；已批准可撤销。金额变更、状态和操作日志同事务提交。
  重复相同操作不重复增减余额；拒绝/撤销必须填写原因。撤销允许余额变负，
  以便追回已经使用的奖励，负余额沿用现有欠费限制。
- 用户仅能查看本人申请和收益，被邀请人身份脱敏。后台可查看完整审计信息。
- 默认关闭新码申请；关闭不删除或失效已有关系、邀请码和待审返利。
- 不自动导入卡网历史邀请或返利，避免旧、新系统同时支付。

## 模块及接口约定

数据模块位于 `backend/internal/invitation`，由 `NewStore(*ent.Client)` 创建。
所有金额 API 用 JSON number，数据库用 NUMERIC，计算用 shopspring/decimal。

```go
type Level struct {
    ID string `json:"id"`
    Name string `json:"name"`
    SpendThreshold float64 `json:"spend_threshold"`
    LifetimeInviteLimit int `json:"lifetime_invite_limit"`
    UnusedInviteLimit int `json:"unused_invite_limit"`
    RebateRate float64 `json:"rebate_rate"`
}
type Settings struct {
    Enabled bool `json:"enabled"`
    ValidityDays int `json:"validity_days"`
    AutoApproveHours int `json:"auto_approve_hours"` // 0 = 仅人工审核，上限 8760
    Levels []Level `json:"levels"`
}
```

- `Settings(ctx) (*Settings, error)`
- `SaveSettings(ctx, Settings, actorID int64) (*Settings, error)`
- `Overview(ctx, userID int64) (*Overview, error)`
- `Apply(ctx, userID int64, requestID string) (*Invite, error)`
- `ListInvites(ctx, userID int64, page, pageSize int, admin bool) ([]Invite, int64, error)`；
  admin=true 时 userID=0 返回全部，非零按用户过滤；admin=false 要求 userID>0 并脱敏。
- `ListRebates(ctx, userID int64, status string, page, pageSize int, admin bool) ([]Rebate, int64, error)`；
  过滤和脱敏规则同上。
- `SetOverride(ctx, userID int64, levelID string, actorID int64) error`；空字符串恢复自动。
- `Review(ctx, rebateID int64, action, reason string, actorID int64) (*Rebate, error)`；
  action 为 approve/reject/revoke。
- `BindRegistration(ctx, transactionalClient *ent.Client, redeemID, userID int64) error`：
  普通管理员邀请码无归属时不处理。
- `CaptureAcquisition(ctx, transactionalClient *ent.Client, redeemID, userID int64) error`：
  无原生邀请关系时不处理；必须与余额到账事务同时提交。
- `HasInviter(ctx, client *ent.Client, userID int64) (bool, error)`：
  供通用 affiliate 避免重复奖励。
- `UnbindRegistration(ctx, transactionalClient *ent.Client, redeemID, userID int64) error`：
  仅供未完成的 OAuth 注册回滚，与恢复邀请码一起执行，有返利后不得解绑。

- `AutoApprove(ctx, limit int) ([]Rebate, error)`：执行一轮超时自动入账，返回本轮入账的记录。
  `StartAutoApprove(interval, credited)` 启动定时任务（服务启动时以 10 分钟间隔运行，
  随进程退出停止）；多实例同时运行安全，入账仍由返利行锁和 pending 状态保证只发生一次。
- `DisableInvite(ctx, inviteID, actorID int64) (*Invite, error)`：管理员作废一个未使用且未过期的
  邀请码，额度退回邀请人；已使用或已过期返回冲突，重复调用无副作用。
- `Overrides(ctx, page, pageSize int) ([]Override, int64, error)`：被管理员指定等级的用户列表。

Overview JSON：`enabled`, `validity_days`, `auto_approve_hours`, `levels`（完整等级阶梯）, `level` (Level),
`recharge_total`, `next_level` (Level/null), `next_level_remaining`,
`override_level_id` (string), `lifetime_used`, `unused_count`,
`summary: {pending, paid, rejected_or_revoked}`。

Invite JSON：`id`, `code`, `inviter_id`, `inviter_email`, `invitee_id` (number/null),
`invitee_email`, `status` (unused/used/expired/disabled),
`created_at`, `expires_at`, `used_at` (string/null)。

Rebate JSON：`id`, `invite_id`, `invite_code`, `inviter_id`, `inviter_email`,
`invitee_id`, `invitee_email`, `source_type` (payment/redeem_code),
`source_id`, `source_currency`, `source_occurred_at`,
`first_amount`, `rebate_rate`, `rebate_amount`, `level_name`,
`status` (pending/approved/rejected/revoked), `review_reason`,
`created_at`, `reviewed_at` (string/null)，
`source_status`（在线支付首付订单的当前状态，卡密为空）, `auto_approved`（是否超时自动入账）。
用户视图隐藏 invitee_id（null）、source_id（0）和 source_status（空），仅管理员可读取原值。

HTTP 使用现有 response envelope 和分页结构：

| 方法 | 路径（相对 /api/v1） | 请求/响应 |
| --- | --- | --- |
| GET | /user/invitations | Overview |
| GET | /user/invitations/codes | 分页 Invite，可筛选 status/q |
| POST | /user/invitations/codes | `{request_id: UUID}` → Invite |
| GET | /user/invitations/rebates | 分页 Rebate，可筛选 status/q |
| GET | /admin/invitations/stats | Stats（全站待审/已入账/未通过笔数与金额、邀请码数量、指定等级人数） |
| GET/PUT | /admin/invitations/settings | Settings |
| GET | /admin/invitations/codes | 分页 Invite，可筛选 user_id/status/q |
| POST | /admin/invitations/codes/:id/disable | 作废未使用的邀请码 → Invite |
| GET | /admin/invitations/overrides | 分页 Override（user_id, email, level_id, updated_at） |
| GET | /admin/invitations/rebates | 分页 Rebate，可筛选 user_id/status/q |
| GET | /admin/invitations/users/:id | Overview |
| PUT | /admin/invitations/users/:id/level | `{level_id: string}` → Overview |
| POST | /admin/invitations/rebates/:id/:action | `{reason: string}` → Rebate |

列表筛选（`Store.SearchInvites` / `SearchRebates`，`ListFilter{UserID, Status, Query}`）：

- `status`：邀请码为 unused/used/expired/disabled，返利为 pending/approved/rejected/revoked；
  其他值返回 400。邀请码状态与列表显示使用同一个 SQL CASE，筛选结果和显示不会不一致。
- `q`：不区分大小写的包含匹配，`%`、`_`、`\` 按字面处理，最长 200 字符。
  用户视图只匹配邀请码；只有管理员视图额外匹配邀请人/被邀请人邮箱，
  否则用户可以用搜索验证被脱敏的被邀请人邮箱。
- 删除仍被用户指定的等级时返回 409 `INVITATION_LEVEL_IN_USE`（原为通用的
  `INVITATION_CONFLICT`），界面据此提示先把这些用户恢复为自动等级。

前端独立入口 `/invitations` 与 `/admin/invitations`，不受 affiliate_enabled 控制。
个人账户显示用户入口；后台一直能配置。分享地址使用
`/register?invitation_code=...`，注册即使未强制要求邀请码也接受显式提供的码；
邮箱验证及 OAuth 路径保留该参数。

## 验证范围

申请幂等及并发额度、注册占用及事务回滚、首次充值与内部支付码去重、
小数舍入、审核/重复审核/撤销原子性、独立鉴权、身份脱敏、页面状态和表单提交。
构建和测试只针对本地工作区；上线及旧系统停止返利另行安排。

## 本地验证记录（2026-11-18）

- PostgreSQL 16：独立临时数据库完整执行全部迁移两次，验证重复启动安全；
  通过真实 AuthService、RedeemService、用户/兑换码/affiliate repository 运行注册、
  首次卡密到账、支付内部充值码、审核重试、撤销及 OAuth 注册回滚。
- `internal/invitation` 的 14 个顶层测试及账户状态变化子用例通过 `-race`：
  包含并发申请额度、同一请求幂等、并发首充、金额舍入、账户转换和审计回滚。
- OAuth 待完成注册的账号创建、邀请绑定、第三方身份绑定和会话消耗共用一个事务；
  请求取消后不会留下可登录但未绑定邀请人的账号。默认订阅、平台配额和普通
  affiliate 初始化在提交后执行，避免独立配额连接看不到新用户。
- 前端已有草稿通过 typecheck、邀请码文件 ESLint、生产构建，以及注册/邮箱验证/
  OAuth 相关 7 个测试文件共 96 个测试；未进行浏览器视觉验收。
- 后端独立审查提出的 OAuth 事务窗口及配额写入时序问题已修复并复查。
- 后端 handler/service/repository/routes 的注册、OAuth、邀请码、affiliate、
  兑换相关回归与 `go build ./cmd/server` 通过；`git diff --check` 通过。

复跑后端事务与真实服务测试：

```sh
cd backend
INVITATION_TEST_DSN='postgres://USER@localhost:5432/postgres?sslmode=disable' \
INVITATION_SERVICE_TEST_DSN='postgres://USER@localhost:5432/postgres?sslmode=disable' \
GOTOOLCHAIN=go1.27.3 go test -race ./internal/invitation -count=1
```

只能指向测试 PostgreSQL；服务联调测试创建并清理独立数据库，测试账号需要
CREATE DATABASE 权限；模块测试创建并清理独立 schema。未设置对应环境变量时，
相关数据库用例会跳过，不能将跳过当作通过。

## 界面

用户最初指定界面由 `gemini-4-carbon` 处理；该模型 2026-11-18 与 2026-11-23 多次调用
均失败（403 账号耗尽、401 Key 已禁用、502 无可用上游），未产生任何修改。
用户随后改为指定 `claude-fable-5-2`，界面于 2026-11-23 由该模型完成，替换了原草稿。

用户页 `/invitations`（`views/user/InvitationsView.vue`）：

- 最新一个未使用的邀请码以票据样式展示，可复制邀请码和注册链接；
  链接为同源 `/register?invitation_code=…`，并带上部署的 base 路径。
- 等级、返利比例、累计有效充值、距下一级差额与进度、两项额度（0 显示为不限）。
- 待审核 / 已入账 / 未通过或已撤销三项金额，以及币种与不换汇的说明。
- 邀请码与首充返利两个分页列表，支持状态筛选和邀请码搜索；被邀请人保持脱敏。
- 暂停申请、额度用完时说明原因并禁用按钮，历史记录照常可读。

申请幂等（`utils/invitationApply.ts`）：request_id 按登录用户存入 localStorage，
直到服务器给出明确结果才清除。无响应、超时、401、408、429、5xx 视为结果未知：
页面保留同一个 request_id，刷新后仍提示“上一次申请没有收到结果”，
只能用同一个编号确认，不会多占额度；此时即使额度显示已满也允许确认
（已满可能正是那次申请造成的）。其余 4xx 是明确拒绝，立即清除。
不同账户使用不同的存储键，不会重放他人的申请。

后台 `/admin/invitations`（`views/admin/InvitationsView.vue`），四个页签：

- 返利审核：默认打开待审核队列。批准、拒绝、撤销都先弹出对话框，写明后果
  （向谁的余额增加或扣回多少）和计算过程（首次金额保留原币种）；
  拒绝和撤销必须填写原因。其他管理员已先处理时提示状态已变化并刷新列表。
- 邀请码：全部邀请码，可按状态、邀请码或邮箱搜索。
- 等级规则：开关、有效期、等级增删改。校验与后端 `validateSettings` 一致，
  错误显示在对应字段旁；已保存等级的标识只读（用户指定等级和历史返利引用它）；
  清空的数字框报“必填”而不是按 0（不限）保存；有未保存修改时离开页面会确认。
- 用户等级：查找个人账户，查看其邀请概况，指定或恢复自动等级，
  并可跳转到该用户的邀请码或返利记录。

顶部汇总来自 `/admin/invitations/stats`，点击待审核金额回到全站待审队列。

## 本地验证记录（2026-11-23）

在隔离的 Linux 沙箱中用 Go 1.27.3、临时 PostgreSQL、Node 22 验证，未连接任何
线上或本机数据库：

- `internal/invitation` 全部 15 个顶层测试通过，包含新增的
  `TestPostgresSearchFiltersStatsAndMasking`（状态筛选、通配符按字面匹配、
  用户不能按邮箱搜索、管理员可以、统计数字）；`-race` 下同样通过。
- handler / routes 的邀请相关测试通过（新增筛选参数不能覆盖登录用户、
  `/stats` 受管理员守卫保护）；service 包的注册、OAuth、邀请码、affiliate、
  兑换相关用例通过；`go build ./cmd/server`、`go vet ./internal/invitation` 通过。
- 前端 `vue-tsc --noEmit`、邀请相关文件 ESLint、`vite build`（输出到临时目录，
  未覆盖 `backend/internal/web/dist`）通过。
- Vitest：新增 4 个测试文件共 49 个用例（工具函数 16、用户页 11、后台页 14、
  注册页原生邀请链接 8），连同原有注册 / OAuth / 路由 / 侧边栏测试共 28 个文件
  288 个用例通过。
- 真实浏览器（无头 Chromium）加载 Vite 开发服务，接口用固定演示数据代替：
  用户页与后台各页签在浅色、深色、390px 手机宽度及英文下渲染，无控制台报错、
  无页面异常、无横向溢出；实际键入验证了小数输入、清空数字框、
  新增等级的标识不被锁定、方向键切换页签。

未做的验证：没有连接真实后端做端到端联调；没有运行后端和前端的完整测试集
（只跑了与注册、OAuth、邀请、兑换、路由相关的部分）；计划中的独立代码审查
因模型网关 502 两次未能启动，本轮新增代码只经过作者自查。

复跑前端邀请相关测试：

```sh
cd frontend
pnpm exec vitest run src/utils/__tests__/invitations.spec.ts \
  src/views/user/__tests__/InvitationsView.spec.ts \
  src/views/admin/__tests__/InvitationsView.spec.ts \
  src/views/auth/__tests__/RegisterView.nativeInvitation.spec.ts
```

## 上线状态

当前只修改本地工作区，未部署。正式启用时先随服务启动执行
`270_native_invitations.sql`，再在后台配置等级、额度、比例并开启新码申请；
默认关闭且返利比例为 0。启用原生返利前需停止旧卡网针对同一批用户的返利，
本模块不会调用或自动停用旧服务器，也不会自动导入其历史记录。

## 第二轮完善（2026-11-23，`claude-fable-5-2`）

对照卡网原实现逐项复查后补齐的差异，以及自查发现的问题：

- 新增超时自动入账（见“规则”），卡网有此行为而第一版没有。默认关闭，需在后台“等级规则”
  里填写小时数才生效。
- 管理员可作废未使用的邀请码。原先原生邀请码被禁止在通用兑换码管理里修改，
  泄露或发错的码只能等它过期。
- 累计额度的统计不再把已作废的邀请码算作占用（此前只有“当前未使用”一项排除了它）。
- 后台返利列表和审批对话框显示首付订单的当前状态；订单已退款或未完成时，
  列表给出醒目标记，批准前再提示一次。此前管理员无从得知首付是否已退款。
- 首付订单的币种快照无法识别时按 CNY 记录（与支付模块的默认值一致），
  不再让这笔充值的入账事务失败。
- 列表每页上限由 100 调整为 1000，与系统表格设置允许的最大每页条数一致；
  此前把全站每页条数设为 200 以上时，邀请列表会直接报参数无效。
- 行锁由 `FOR UPDATE` 改为 `FOR NO KEY UPDATE`：只改余额不改主键，
  不再阻塞其他事务对同一用户的外键检查（例如该用户同时在兑换卡密）。
- 用户页展示全部等级的门槛、返利比例和额度，并标出当前等级；开启自动入账时说明等待时长。
- 后台“用户等级”页列出所有被指定等级的用户，可直接点开；此前只显示一个人数。
- 后台两张表格在 1280 宽度下曾有列被固定的操作列遮住，已调整为两行显示并把状态并入返利列。
- 错误提示里的分隔符改为随语言变化（中文全角冒号），不再使用破折号。

### 验证记录

隔离的 Linux 沙箱，Go 1.27.3、临时 PostgreSQL、Node 22，未连接任何线上或本机数据库：

- `internal/invitation` 在 `-race` 下 22 个顶层测试全部通过、无跳过，其中 7 个为本轮新增：
  自动入账的等待时长与并发只入账一次、需人工处理的六种情况不被自动入账、
  定时任务入账后可停止、作废邀请码退回额度且不可再用于注册、指定等级用户列表与等级阶梯、
  无法识别的币种不阻塞入账。
- handler / repository / routes / cmd/server 的邀请、注册、OAuth、affiliate、兑换相关测试通过；
  `internal/service` 同一筛选下 441 个用例通过；`go build ./cmd/server`、`go vet`、`gofmt` 通过。
- 前端 `vue-tsc --noEmit`、邀请相关文件 ESLint、`vite build`（输出到临时目录）通过；
  中英文各 211 个文案键，完全一致。
- Vitest：28 个文件 301 个用例通过，其中邀请相关 4 个文件 62 个用例（本轮新增 13 个）。
- 反向验证：分别破坏“首付异常标记”“作废前确认”“自动入账检查订单状态”三处逻辑，
  对应测试均失败，恢复后通过。
- 无头 Chromium 渲染 34 张截图（浅色、深色、390px、1280–1920 宽度、中英文）：
  无控制台报错、无页面异常、无横向溢出，1280 及以上宽度表格不超出容器。

仍未做的验证：没有连接真实后端做端到端联调；没有运行后端和前端的完整测试集；
没有在真实支付渠道上验证退款后的订单状态取值（代码按 `payment/types.go` 中的状态常量判断）；
计划中的独立代码审查本轮第三次因模型网关 502 未能启动，新增代码仍只经过作者自查。
1024 宽度下后台表格需要横向滚动，与系统其他后台列表的行为一致。
