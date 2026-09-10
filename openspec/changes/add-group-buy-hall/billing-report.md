# 月卡准入与结算实施记录

更新时间：2026-09-10。工作区保留 `custom/0.2.0`，未提交、未推送。未修改任务开始前已有的 pricing_service.go、pricing_service_test.go、gateway_record_usage_test.go 改动。

## 已实现

- `monthcard.Store.Admit` 在旧订阅缓存之前执行；无该用户分组月卡历史时返回 nil 继续原路径。有历史则按持久化顺序选取当前有效且有可用配额的新卡和旧订阅，并冻结候选与各自周期。所有 Key 共享数据库用量和顺序。
- 通用及 Google API Key 鉴权、网关预检查均读取真实账户欠费状态。订阅分组没有权益时禁止余额回退；余额仅补扣已发生调用的差额。
- `UserSubscription.MonthCardSnapshot` 传递请求快照；聚合订阅 ID 为零。Anthropic/Gemini/Antigravity 通用扣费、OpenAI 扣费和 live identity 均避免把新卡 ID 写入 legacy subscription 外键。
- `SettleTx` 并入现有统一 `usage_billing_dedup` 事务；先锁用户，再按 ID 预锁全部候选卡，实际按用户顺序分摊。卡周份额和整卡份额均原子封顶，差额完整扣余额，允许负余额。
- 金额使用 decimal 量化至 NUMERIC(20,8)，分摊合计与实际费用一致。幂等指纹包括固定候选、顺序、周期、旧订阅限额及重置代次，JSON 往返保持稳定。
- 卡到期不改变在途请求归属；退款/撤销使剩余应付金额转余额。实际结算检查支付订单退款状态。API Key 在请求途中改绑分组不改变原快照：结算验证固定用户归属及每个候选的原分组，不要求 Key 当前分组与原分组一致。
- 周期历史独立持久化。旧订阅保留自然日日限额、一次性日额度与滚动周/月窗口；历史周期完成只更新对应历史用量，不污染新窗口。
- 手动重置旧订阅使用独立 generation：API 重置事务按用户→旧订阅锁序执行，每次显式重置（包括当前用量为零）均推进对应代次。DB trigger 兼容旧仓库直接降低计数的同时间戳重置。旧请求与新额度分别结算。
- 月卡扣费先保存完整不可变命令至 SQL pending 表，事务成功后原子删除。失败不走旧订阅/全额余额非事务回退。`StartMonthCardBillingRecovery(repo)` 周期重放，进程重启后继续使用原快照；测得费用在失败日志中保留，避免恢复成功后账单仍显示零费用。
- OpenAI WebSocket native/passthrough 后续 turn 均在 `BeforeRequest` 重新准入。每个 turn 单独捕获快照并使用独立强制扣费 ID，异步 worker 不读取下一轮订阅。
- Grok 异步视频将创建时卡快照/旧订阅 ID、模型、倍率、时间与路由归属持久化到 SQL。完成轮询恢复创建快照，月卡采用数据库幂等而不依赖 Redis 一次性 claim；SQL 路由归属可恢复被 Redis 驱逐的绑定。视频读取端点按已有任务归属校验，欠费或卡过期不阻止读取和完成结算。
- Key 可绑定分组列表包含有效月卡，已耗尽当前周期或欠费仍允许配置 Key。扣费明细按最近100个完整请求返回，避免截断单请求分摊后展示错误合计。

## 数据库与注入

迁移 `239_month_card_billing.sql` 新增周期历史、legacy 重置代次/trigger、分摊、待恢复扣费命令、异步视频创建快照表。核心迁移由核心代理维护。

`SubscriptionService`、`BillingCacheService`、`APIKeyService` 提供 `SetMonthCardStore(*monthcard.Store)`。主代理已装配 Wire。`service.StartMonthCardBillingRecovery(UsageBillingRepository) func()` 返回停止函数；应在 SQL pool 关闭之前调用，主代理已接入清理流程。

## 验证结果

真实 PostgreSQL 16，临时本地服务；每个测试创建并销毁独立 schema，实际执行新迁移及生产统一扣费仓库，不使用金融事务模拟。

```sh
MONTHCARD_BILLING_TEST_DSN='postgres://varrge@127.0.0.1:55439/postgres?sslmode=disable' \
GOTOOLCHAIN=go1.27.0 go test ./internal/monthcard -run TestMonthCard -count=1
```

最终通过（2.377s）。覆盖：跨卡/余额分摊、欠费、无权益阻断、20路并发跨 Key 周额度封顶、幂等冲突、事务回滚、持久命令恢复、跨周/跨到期、固定排序、旧订阅混合历史周期、改绑 Key、视频快照归属、手动同零点日重置（已用4及已用0两种在途情形）。此前真实数据库并发套件 `-race` 亦通过（3.832s，重置代次变更前）。

定向 `-tags unit` 服务/仓库/中间件/handler 的订阅、统一扣费及 WS 测试全部通过：service 42.498s，repository 5.263s，middleware 2.703s，handler 10.762s。此轮在最后 generation 修改之前执行；主代理正在执行全量 unit 套件，最终总体结果以主代理报告为准。

新增服务回归验证：聚合对象即使误带非零 ID 也不产生 legacy FK；缺少事务仓库时显式失败；卡快照不借用旧分组限额缓存；指纹区分候选顺序。视频快照 JSON/恢复测试、只读路由豁免边界测试通过。

最后 generation 修改后的定向服务/handler/中间件复验通过：service 1.858s、handler 0.928s、middleware 2.535s；`git diff --check` 通过。

## 已交接主代理的范围

- Gemini 批量图片是原有独立余额 hold/capture 体系，本次保留既有计费方式，尚未适配月卡。主代理已向用户补充询问范围并说明当前行为，尚未收到选择；普通图片调用已接入月卡。
- 原 OpenAI live 功能本身仍有零费用实现边界；本任务按主代理指示只修复 synthetic legacy ID，不重构既有 live 计费。
- Grok 视频沿用“状态/内容查询首次观察到完成后结算”的原流程；SQL快照与结算命令可以持久恢复，但本变更未引入独立主动轮询上游任务的后台 worker。
- PostgreSQL 完全不可用、上游已经产生费用、且命令尚未成功持久化时，无法用同一不可用数据库保证恢复。此路径明确返回/记录错误并保留测得费用，不会静默走非事务回退；不声称消除此类基础设施故障窗口。
