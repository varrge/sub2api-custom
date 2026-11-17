# v0.2.22 合并后的充值与退款复验

日期：2026-11-17（Asia/Shanghai）。被测提交：`037bd3dd72e950e47f9346b7a9b97b7ce9a2880d`。自定义基线：`4e4f957af39e43d85263a9504553b845bea9ad40`。

## 结论

后续状态：用户要求修正后，四项问题及独立复核发现的相关边界已处理。最终修正和验收见 [payment-fixes.md](payment-fixes.md)；下文保留修正前的故障证据。

本轮没有确认合并新增的充值回归，但发现并复现了四项既有支付缺陷，不能给出“充值与退款无阻断问题”的验收结论。建议先修复并验证这些资金状态问题，再发布。

本轮只验证并记录，没有修改支付业务代码，没有推送、打 tag 或更新美国生产。中国验收环境保持原候选镜像。本文补充并更新此前 implementation.md 中“独立复核无阻断项”的阶段性结论。

## 实测通过

本机从被测提交构建完整应用，使用单独 PostgreSQL 18.1、Redis 8.4、空白数据库和测试账号。通过真实管理员/用户 HTTP 接口配置支付、创建订单、提交验签回调、检查数据库到账结果。网关为本地模拟 EasyPay，没有使用真实商户或转账。

| 场景 | 支付金额（人民币） | 实际入账余额（美元） | 结果 |
|---|---:|---:|---|
| 普通充值 100，倍率 0.14，手续费 2% | 102 | 14 | 通过 |
| 充值 1000，倍率 0.14，赠送 30%，手续费 2% | 1020 | 182 | 通过 |
| 企业充值 1000，独立倍率 0.2，排除个人赠送，手续费 2% | 1020 | 200 | 通过 |
| 充值 1000，倍率 0.14，优惠 20%，手续费 2% | 816 | 140 | 通过 |
| 上述各订单 24 次并发重复回调 | 不新增支付 | 不重复入账 | 通过 |
| 已签名但金额错误的回调 | 0.01 / 应付 100 | 0 | 拒绝入账 |
| 已取消订单随后收到合法付款通知 | 100 | 14 | 完成并入账 |
| 普通充值全额退款 | 渠道退款 102 | 14 → 0 | 一次网关退款、订单 REFUNDED |

自动化复跑：

- 后端支付、充值、退款、兑换与月卡规则专项：425 项通过（含子测试；217 个顶层测试），0 跳过。
- PostgreSQL 支付回调安全、月卡支付/退款恢复、共享额度竞态：36 个唯一测试/子测试连续三轮通过，共 108 次通过事件，0 跳过，启用 `-race`。
- 前端支付相关：23 个文件、218 项通过。
- 两个额外的故障复现断言成功触发旧缺陷，不计作正确性通过。

后端专项命令：

```sh
# 指向本轮专用 PostgreSQL；不是生产 DSN。
MONTHCARD_TEST_DSN="$TEST_DSN" MONTHCARD_PAYMENT_TEST_DSN="$TEST_DSN" \
go test -json -tags=unit ./internal/service ./internal/handler \
  ./internal/handler/admin ./internal/payment/provider \
  -run 'Test.*(Payment|Recharge|Refund|Redeem|MonthCardOrderConsent|Admin.*Balance)' \
  -count=1 -timeout=15m

PAYMENT_WEBHOOK_TEST_DSN="$TEST_DSN" MONTHCARD_PAYMENT_TEST_DSN="$TEST_DSN" \
MONTHCARD_BILLING_TEST_DSN="$TEST_DSN" \
go test -race -json ./internal/integration ./internal/monthcard \
  -run 'TestEasyPayWebhookSecurityPostgres|TestMonthCardPaymentFullMigrationsFulfillmentRefundAndRecovery|TestSharedCredit' \
  -count=3 -timeout=15m
```

## 四项已复现的问题

### 1. 过期较久的订单收到成功付款回调后不入账（P1）

位置：`backend/internal/service/payment_fulfillment.go:208`、`:260`。

复现：正常创建余额订单，模拟订单已过期十分钟，再提交通过验签、金额及商户校验的成功回调。HTTP 返回 `success`，但订单仍为 EXPIRED，余额为 0，paid_at 和 payment_trade_no 为空；管理员补单返回 `400 order is not paid`。

原因：普通、账户类型未变化的 EXPIRED 订单只允许在五分钟宽限内转 PAID；超出后仅写审计就确认回调成功。未创建退款任务。关单网络失败时也能形成此路径。

修复边界：合法付款必须先持久化，再履约或退款；保留企业账户类型转换保护和已完成/已退款订单的幂等保护。

### 2. FAILED 订单补到账时遗漏付款字段，后续可显示已退款但未请求网关（P1）

位置：`backend/internal/service/payment_fulfillment.go:258`、`backend/internal/service/payment_refund.go:398`。

复现：创建订单后模拟供应商建单请求失败，保留 FAILED、paid_at=NULL、payment_trade_no=''，再提交合法付款回调。余额从 0 → 14，订单 COMPLETED，但付款字段仍为空。

随后通过管理员退款 API 处理该订单，返回 success、余额扣回、订单 REFUNDED；审计明确记录 `REFUND_NO_TRADE_NO {"detail":"skipped"}`。此时模拟网关已经关闭，也未阻止系统记录退款成功。

原因：FAILED 回调跳过保存付款事实，直接履约；空流水退款路径直接认定成功。

修复边界：区分未确认付款的 FAILED 与已付款履约失败的 FAILED；经过合法回调先原子保存付款事实。不能放开对未付款订单的人工直接充值，也不能将缺失流水直接当成退款成功。

### 3. 网关已接收退款但响应断开，本地会恢复余额并允许重复退款（P1）

位置：`backend/internal/service/payment_refund.go:391`、`:421`、`:802`；`backend/internal/payment/provider/easypay.go:585`。

完整 HTTP 复现：模拟网关先记录已接受退款，再关闭连接。本地收到 EOF 后返回 rolled back，订单恢复 COMPLETED，余额从已扣除的 0 恢复为 14，用户又有可用余额。

独立 Go overlay 补充复现：初始钱包 100，网关接收 40 后丢响应；本地恢复 100。再次退款 40，网关累计接收 80，本地钱包最终 60，仅记录一笔 40 退款。该补充使用内存 SQLite 与 mock 钱包，未冒称真实资金退款。

原因：请求后的不确定结果被当成明确失败回滚；EasyPay 没有实现退款查询或适配器级退款幂等能力。

修复边界：区分确定失败与结果未知；未知结果保留已扣余额及待确认状态。保存原请求信息，通过查询或人工核实收敛，不能自动重发非幂等退款。

### 4. 普通余额退款中断后卡在 REFUNDING，现有入口不能恢复（P1）

位置：`backend/internal/service/payment_refund.go:337`、`:353`、`:474`；`backend/internal/service/payment_order_expiry_service.go:99`。

独立复现：真实 ExecuteRefund 提交认领并完成扣减后，在供应商调用处注入 panic。内存 SQLite 中订单停留 REFUNDING，mock 钱包 100 → 60，但 refund_amount=0、refund_reason=NULL；PrepareRefund、QueryAndFinalizeRefund 和人工确认入口均拒绝恢复。

原因：普通余额退款认领、扣减、远端调用与终态分开提交，未保存足够的恢复信息；现有周期恢复与人工确认只覆盖月卡。

修复边界：持久化退款请求和实际扣减，并提供中断后的查询/确认路径。存量缺少扣减记录的订单必须核对，不能盲目再扣款或再退款。

## 为什么不是本次合并新引入

四项问题所涉及的 `payment_fulfillment.go`、`payment_refund.go`、`payment_month_card.go`、`payment_order.go`、`provider/easypay.go` 在自定义基线与本次合并提交的 Git blob 完全相同。支付 handler、redeem_service、user_repo 和鉴权缓存失效器也没有生产差异。

本轮另外检查了合并祖先关系、差异空白检查、冲突标记与支付依赖连接，未发现合并遗漏。唯一类似冲突分隔符的匹配位于原有字符串内。中国验收应用仍 healthy、RestartCount=0，二进制校验和与上一轮一致。

全量后端、迁移、调度/扣费、前端构建及 1092 项关键测试等上一轮结果见 implementation.md；本轮未重复这些已通过且源码未变化的检查，未将它们计作本轮新增测试。

## 证据与范围

- 完整 HTTP 演练：`/tmp/sub2api-022-recheck/http_rehearsal.py`、`http-rehearsal.log`、`http-results.json`。
- 自动化日志：同目录 `payment-unit.jsonl`、`payment-db-race.jsonl`、`frontend-payment.log`。
- 独立复核：`/tmp/sub2api-022-recheck-payments-review.md`。
- 退款故障复现：`/tmp/sub2api-022-payment-review-reproduction.log`、`/tmp/sub2api-022-payment-review-overlay_test.go`。

没有扫描美国生产历史资金数据，不能据此断言线上已有用户受损。没有真实转账、真实供应商退款或支付页面浏览器交互；跨鉴权缓存的真实上游消费与退款并发也未在本轮演练。

专用应用、两个专用数据库容器及临时登录凭据在验收后清理；保留不含真实凭据的测试脚本和日志供修复回归使用。
