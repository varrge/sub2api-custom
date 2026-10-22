# 企业模式实施记录

> 以下记录为旧基线验收历史；2026-10-22 的 `custom/0.2.17` 移植与发布见 [port-0217.md](port-0217.md)。

用户已授权开始实现。业务规则见 design.md，最新第三轮规则优先。保留当前工作区已有改动；开发验证完成后，按用户追加授权部署中国独立验收实例并打开本机 18082 隧道，见 [deployment-cn.md](deployment-cn.md)。未提交或正式发布。

## 前后端契约

- 用户 DTO 增加 `account_type: 'personal' | 'enterprise'`；管理员用户 DTO 增加 `enterprise_group_rates: Record<string, number>`。管理员创建支持 `account_type`，更新支持 `enterprise_group_rates: Record<string, number | null>`（null 清除）；已有用户修改身份走独立转换接口。
- `POST /api/v1/admin/users/:id/enterprise/preview` body `{conversion_rate?: number}` → `{user_id, account_type, balance, cards:[{id,code,group_id,group_name,total_quota_usd,total_used_usd,remaining_usd,expires_at}], remaining_usd, conversion_rate, credited_usd, balance_after, preview_token, blockers: string[]}`。额度只读，无月卡不要求折算倍率。
- `POST /api/v1/admin/users/:id/enterprise/convert` body `{account_type, conversion_rate?, preview_token, operation_id}` → 更新后的管理员用户 DTO。企业改回个人也用该接口；重试复用 operation_id。409 重新预览，不能自动替用户确认。
- 模型广场 group DTO 新增 `enterprise_rate_multiplier?: number | null`；保留普通 `user_rate_multiplier`。企业每次进入默认勾选“仅看企业用户倍率”，只展示显式配置该字段的组；取消后原有列表/普通计价。切换不更改后端计费。
- 支付管理员配置增加 `enterprise_balance_recharge_multiplier`（默认 1）；系统设置接口通过 `payment_enterprise_balance_recharge_multiplier` 读写同一配置，设置页面沿用现有统一保存流程。用户 checkout 原 `balance_recharge_multiplier` 返回后端选择后的实际倍率。企业只显示余额充值。
- `GET /api/v1/usage/monthly-export?month=YYYY-MM&kind=all|summary|details|conversions&group_id?&api_key_id?` 默认 all 返回同一数据库快照的 ZIP（三份 CSV）；其余返回单份 CSV，单独下载不保证跨文件快照一致。完整包不能按密钥筛选；在用量页面放月份与导出按钮，复用 API 客户端鉴权，blob 下载。文件为消费汇总、消费明细或折算入账。
- 企业最多六个一级入口：/usage、/monitor、/keys、/purchase、/profile、/tickets。支付关闭时隐藏 /purchase，工单未开启时隐藏 /tickets，复用现有功能开关且随公共设置更新。模型广场从渠道状态进入，订单从充值页进入；低余额通知继续原功能。

## 文件分工

- Kimi K3 YOLO：frontend 展示页面和组件、需要的前端类型/API 方法、国际化与对应 UI 测试。不要修改 frontend/src/router/index.ts、frontend/src/stores/auth.ts 或任何 backend 文件；这两处前端权限逻辑及所有后端由主代理负责。允许 AppSidebar 按 account_type 分支企业导航、AppLayout 企业引导适配。
- 主代理：后端全部实现、前端路由和 auth store、最终接线和集成检查。

## 进度

- [x] 保存修改前工作区快照，确定接口契约。
- [x] 用户类型、独立企业倍率及持久化。
- [x] 企业权限、准入及计价。
- [x] 企业充值与活动隔离。
- [x] 月卡转换预览、原子入账与幂等。
- [x] 模型广场报价接口及月度导出后端。
- [x] 前端身份状态、路由限制及对应回归测试。
- [x] 后端单元/真实数据库集成验证和独立审查。
- [x] 模型广场与月度导出 UI。
- [x] Kimi K3 YOLO UI、前端权限接入。
- [x] 前端完整构建、单元测试和独立审查。

## 实现边界与验收记录

- 用户设置与身份转换是两个明确的保存动作；转换只原子提交类型、月卡终止、余额及折算审计，不隐式保存用户编辑里的草稿。允许个人用户预先保存企业报价，未配置时转换后沿用现有倍率。
- 请求准入在数据库保留转换保护记录；正常结算后释放。进程异常退出时保护记录不会自动超时放行，需先核对未结算任务再由运维清理，避免重复折算。转换界面会报告尚未完成的调用。
- Live/WebRTC 当前计费链路尚未具备企业价格语义，企业密钥禁用该能力；普通用户原有行为保留。
- 旧链接在身份切换后收到验证成功的付款，保存为待退款并禁用未兑换的充值码；退款及异步查询使用持久化“未入账”标识，不能扣除原有余额或旧订阅。
- 已折算月卡退款使用现有二次确认，明确提示冲回金额及余额不足会形成欠费；逐卡冲回幂等，刷新余额缓存。
- 历史日志保留原始记录，不按当前价格重算。新批量图片账单记录基础费用、实际倍率和实收；原有历史日志本身缺失的数据不能追溯补造。
- PostgreSQL 集成验证：并发转换幂等、跨分组额度合计、重复转换/退款、预览变更、用户锁与月卡锁等待跨到期、月度数据隔离及 ZIP。

## 验证结果（开发环境）

- 后端 service、handler、admin、DTO、middleware、routes、repository 全包 unit 标签测试通过（`/tmp/sub2api-enterprise-final-unit-tests.log`）。
- 真实 PostgreSQL 的企业 service 集成、repository 完整迁移与事务测试通过；月卡 core/billing/payment 全套事务测试通过。
- 企业旧 JWT 会话权限隔离测试通过；历史订阅延期/额度重置/恢复拒绝企业测试通过；未入账付款退款恢复和折算退款二次确认测试通过。
- 后端可编译；最终 UI 接入后的完整 Vue 类型检查、生产构建及全量测试通过。
- 模型广场组件测试、真实组件内存回归已通过，覆盖默认企业报价、恢复普通报价、切换后清理隐藏筛选，以及小倍率分时价格精度。
- 转换弹窗 12 项测试通过；已合入的导航、充值、导出和弹窗相关 95 项测试通过。系统设置企业充值倍率的默认值与独立保存/读取回归通过。
- 最后配置界面与支付测试 48 项通过；分组配置新增跨用户加载失败、乱序返回隔离，专项 5 项及独立真实组件 5 项回归通过。最终独立复核无遗留 P1/P2。
- 系统设置充值倍率及接口契约测试通过；设置页面和价格格式化共 44 项回归通过。全部最终文件合入后的前端生产构建通过，仅有既有包体积与 Browserslist 数据提示。
- 最终前端全量测试：286 个测试文件、2092 项测试全部通过（`pnpm test:run --maxWorkers=2 --minWorkers=2`，日志 `/tmp/sub2api-enterprise-frontend-final-tests.log`）。最终构建日志 `/tmp/sub2api-enterprise-frontend-final-build.log`；`git diff --check` 通过。
- 临时 PostgreSQL 测试容器已移除，仅包含本次生成的测试数据；工作区恢复备份与 stash 保留。Kimi 上游曾发生 429/502/超时，任务在相同 Kimi K3 YOLO 模式恢复并完成，未切换 UI 作者模型。
- 开发中 UI 子代理为排查类型错误执行了 stash，暂时覆盖工作区；已从完整 stash 恢复195 tracked文件，并合并期间写入的3个UI文件。修复前后均有完整备份，之后 UI 在独立副本完成。开发阶段未提交/未部署；后续中国验收部署见独立记录。

## 管理员验收路径

1. 在用户管理中为测试账户保存分组授权与企业倍率。保留一个未填企业倍率的可用余额分组，用于验证原倍率回退。
2. 选择转换为企业用户。有有效月卡剩余额度时，核对全部月卡合计、输入折算倍率并确认预计余额；没有剩余额度时直接确认身份转换。存在未结算调用、支付或退款时先处理提示的阻塞项，再重新预览。
3. 用该账户登录，核对最多六个业务导航、旧活动链接跳转和仅余额充值。支付关闭隐藏充值，工单未开启隐藏工单。普通充值倍率与企业充值倍率分别配置，企业默认倍率应为 1。
4. 从渠道状态进入模型广场，默认只显示有企业报价的分组；取消勾选后查看普通报价。实际调用有企业报价的组使用企业倍率，未配置的组沿用现有倍率。
5. 在使用记录中导出一个有消费的月份，完整包包含汇总、明细和折算三份 CSV；完整包和折算文件不应用 API Key 筛选。核对消费金额及月卡转余额记录，不把折算当作现金充值。
