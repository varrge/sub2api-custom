# v0.2.22 充值与退款修正验收

日期：2026-11-17（Asia/Shanghai）。修正基于 `037bd3dd72e950e47f9346b7a9b97b7ce9a2880d`，工作分支 `codex/merge-upstream-0.2.22`。用户要求修正前次复验的四项支付问题。

## 最终行为

1. EXPIRED 超过原五分钟宽限、以及尚未记录付款事实的 FAILED 订单，收到通过验签、金额与商户校验的成功通知后，先持久化付款时间及渠道流水，再幂等履约。已退款订单不会重新入账；个人订单在账户转为企业后仍进入退款处理，不错误充值。
2. 缺少渠道付款流水的退款明确拒绝，不再跳过渠道请求却显示成功。
3. 普通余额退款在同一事务中认领订单、扣除实际可用余额并保存 `REFUND_STARTED`。提交后立即失效余额缓存，再请求支付渠道。渠道受理后丢响应、超时、未知响应均保留扣减及待核实状态；不能重复发退款。只有明确拒绝或核实失败才归还实际扣减，归还不增加累计充值。
4. 中断退款保存原金额、渠道金额/币种、实际扣减和请求时间。超时 `REFUNDING` 可查询恢复；查询复用原商户并按渠道金额请求，未知结果继续等待。余额恢复队列按 ID 轮转，前面长期待人工的订单不会永久挡住后续订单。
5. 管理员可对余额退款明确核实“成功”或“失败”，填写依据并确认；请求携带原 `updated_at`。已有审计金额不可被客户端覆盖；缺审计的存量订单必须填写经核对的退款额度及已扣额度，未扣必须明确填 0。归还失败会回滚整个事务、保留可重试状态，界面不会误报核实完成。旧月卡全额退款确认行为保留。
6. 原退款响应、查询与人工确认按订单版本提交终态，旧尝试晚返回不能覆盖新尝试，也不能再次归还或扣减。EasyPay 仅明确解析的“订单不存在”业务拒绝允许改标识重试；Stripe 缺本次退款号时要求人工核实，不再取其他历史退款。

## 完整应用验收

使用新构建的本地应用、独立 PostgreSQL 18.1 / Redis 8.4、空白数据库、本地 EasyPay 模拟网关和一次性账号。通过真实登录、管理员/用户 API 与验签回调，共 28 项断言全部通过。

| 场景 | 渠道实付（CNY） | 入账余额（USD） | 结果 |
|---|---:|---:|---|
| 普通充值100、倍率0.14、手续费2% | 102 | 14 | 通过 |
| 充值1000、倍率0.14、赠金30%、手续费2% | 1020 | 182 | 通过 |
| 企业充值1000、独立倍率0.2、手续费2% | 1020 | 200 | 通过 |
| 充值1000、倍率0.14、优惠20%、手续费2% | 816 | 140 | 通过 |
| 各笔24次并发重复回调 | 无额外支付 | 无重复入账 | 通过 |
| 取消/过期十分钟/建单失败后收到有效回调 | 100 | 14 | 到账且付款字段完整 |
| 普通全额退款 | 退102 | 14 → 0 | 一次请求，状态REFUNDED |
| 渠道受理后断连接 | 退100，结果未知 | 保持0 | 禁止重复退款 |
| 明确核实退款失败 | 不另发退款 | 0 → 14 | 累计充值不变，重放不能再次归还 |
| 明确失败后重新退款 | 退100 | 14 → 0 | 正常完成，保留各轮审计 |

还验证了已签名但金额错误的回调被拒绝、缺确认勾选/结果被拒绝、伪造已扣金额被拒绝。

## 自动化与独立复核

- 支付、充值、退款、兑换、月卡同意规则、管理员余额相关后端专项使用 `-race`：499项通过（含子测试），0跳过。
- 真实 PostgreSQL 全迁移回归：60项测试通过（含子测试），0跳过；余额退款20个场景、webhook24个场景，另含月卡支付退款恢复与13项共享额度回归。退款覆盖审计故障使状态/钱包整体回滚、网关看到提交后的扣减、实际持有为0或部分余额、8并发确认仅应用一次、同单两次EOF后人工失败再第三次成功，以及返利/充值审计仍然去重。
- 月卡旧测试曾依赖“空渠道流水直接退款成功”，已改为真实本地EasyPay及完整流水。满团自动退款和管理员退款共两次真实HTTP，人工核实与恢复不新增退款请求，权益断言保留并通过。
- 前端5个文件56项测试、typecheck、改动文件ESLint、生产构建通过。构建仍有原有的大chunk提示。
- 独立只读复核未留下阻断项；独立故障注入复验5个场景通过。反馈涉及缺审计自动查询、人工归还失败误报完成、恢复队列饥饿和旧订单类型错误显示查询入口，均已修复。
- `git diff --check` 通过；`golangci-lint v2.14.0` 全量检查通过，0 issues。

命令（TEST_DSN 必须指向可创建临时数据库的隔离 PostgreSQL）：

```sh
MONTHCARD_PAYMENT_TEST_DSN="$TEST_DSN" \
go test -race -json -tags=unit ./internal/service ./internal/handler \
  ./internal/handler/admin ./internal/payment/... \
  -run 'Test.*(Payment|Recharge|Refund|Redeem|MonthCardOrderConsent|Admin.*Balance|EnterpriseLate)' \
  -count=1 -timeout=10m

PAYMENT_WEBHOOK_TEST_DSN="$TEST_DSN" MONTHCARD_PAYMENT_TEST_DSN="$TEST_DSN" \
MONTHCARD_BILLING_TEST_DSN="$TEST_DSN" \
go test -race -json ./internal/integration ./internal/monthcard \
  -run 'TestEasyPayWebhookSecurityPostgres|TestBalanceRefundRecoveryPostgres|TestMonthCardPaymentFullMigrationsFulfillmentRefundAndRecovery|TestSharedCredit' \
  -count=1 -timeout=8m
```

## 迁移与发布

完整HTTP发现迁移131的全局 `(order_id, action)` 唯一约束使第二次退款审计失败。新增 **268_payment_refund_attempt_audits.sql** 只放开 `REFUND_` 审计的多次记录，保留所有非退款审计的唯一性。返利认领SQL改用无目标 `ON CONFLICT DO NOTHING`，实际数据库已验证返利与充值去重仍有效。

迁移268与新二进制须一起发布。旧二进制中显式指定冲突目标的返利SQL不兼容新部分索引，**不能只切回旧镜像**；发布前必须备份数据库，回滚须使用匹配的数据库备份或经过验证的索引兼容方案。未修改任何已有迁移文件。

本次完成本地修正与隔离验收，未推送、打tag、触发Action或更新美国生产；中国验收仍为此前候选镜像。没有核查或更改生产历史账务，没有真实资金退款。支付渠道结果使用模拟响应，真实渠道上线仍依赖商户配置；未做浏览器交互或真实上游消费与退款缓存并发压测。原旧订阅软删除后的恢复机制不在本次普通余额原子恢复修正范围。

## 证据

临时证据目录 `/tmp/sub2api-022-fix/`：

- `http-final/http_rehearsal.py`、`http-final/http-results.json`：最终应用28项检查；早期HTTP日志保留索引故障证据。
- `payment-unit-final.jsonl`、`payment-db-final.jsonl`：最终专项测试。
- `providers-report.md`、`postgres-report.md`、`ui-report.md`、`final-review.md`。
- `reviewer-reproduction.log`、`reviewer-revalidation.log`、`payment-red.log`、`refund-red.log`：修正前后证据。
- `frontend-build.log`、`lint-final.log`。

验收后的专用应用、数据库容器和临时凭据已清理；保留测试代码、报告及脱敏日志。
