# API Key 费用限额错误

API Key 的 5 小时、1 天、7 天费用窗口继续返回 HTTP 429，但不再与 RPM 共用 `rate_limit_exceeded`。

| 窗口 | 业务错误码 |
|---|---|
| 5 小时 | `api_key_5h_quota_exceeded` |
| 1 天 | `api_key_1d_quota_exceeded` |
| 7 天 | `api_key_7d_quota_exceeded` |

OpenAI / Anthropic 风格响应的 `error.type` 为 `billing_error`，`error.code` 为上表错误码。鉴权分组选择入口保留原有顶层 `code` / `message`，同时增加 `error` 对象；Google 响应保留数字 `error.code=429` 和 `RESOURCE_EXHAUSTED`，业务码放在 `error.details[].reason`。已开始的 Responses 流使用原有 `response.failed` 终止事件，其 `response.error.code` 同样区分窗口。

`message` 包含限额、当前已结算用量，以及在有有效窗口起点时的重置时间（UTC，`Z` 后缀）。预留导致超限时，文案为“剩余消费额度不足”，单列“含本次请求的在途预留”；预留金额不作为已经扣费的金额。缺失或已过期的窗口不编造重置时间。

示例：

```json
{
  "error": {
    "type": "billing_error",
    "code": "api_key_7d_quota_exceeded",
    "message": "该 API Key 的 7 天消费额度已用完（限额 $600.00000000，已用 $600.12500000）；本窗口重置时间 2026-11-03T00:00:00Z。请调整此 Key 的费用限额或等待窗口重置；立即重试无效"
  }
}
```

运维和用户错误请求页将这些新记录归为费用额度/余额/订阅，标记为本站业务限制，不归为上游限流。历史记录不回写。

客户端应按业务码区分费用额度与请求频率：已用完时停止短间隔自动重试；预留不足时可以降低预算、调整限额或等待在途请求结算。这里不返回固定几秒的 `Retry-After`，也不宣称能控制只看 HTTP 429 的第三方客户端自动重试。

额度检查边界、费用累加、缓存/数据库读取策略、RPM 和上游错误处理均不变；不新增数据库迁移。
