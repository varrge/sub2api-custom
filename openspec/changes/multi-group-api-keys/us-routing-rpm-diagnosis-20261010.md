# 美国生产多分组 503 排查（2026-10-10）

## 证据

只读检查，美国运行版本为 `0.2.15-custom.4`。未修改生产配置、数据库、Redis 或部署。

首次查询最近 48 小时：模型 `deepseek-v4.1-flash` 的 2,559 条 503 均为同一个多分组 Key 的路由拒绝；没有选中账号、上游状态或上游尝试记录。错误合并了四个分组的排除原因，而非四次上游调用。日志归属首个分组不能证明该分组被调用。

同一窗口该 Key 有 1,335 条成功用量记录，均为最后的 DeepSeek 分组、同一个账号。排查时该分组三个账号中一个启用调度。

DeepSeek 分组的每用户 RPM 为 5，受影响用户无分组 RPM 覆盖。18:45、18:46、18:47、18:50、18:51、18:52 的用量记录各为 5 条；请求完成时间可能跨分钟。18:50–18:52 的 503 集中在每分钟后半段，无账号选择记录。与每分钟限流吻合；历史错误已丢失具体预检查原因，不能断言每一条历史 503 均由 RPM 触发。

## 可复现缺陷

`CheckAPIKeyGroupRoutingLimits` 正确返回 `ErrGroupRPMExceeded`（429），但 handler 将非全局业务限制转换为 `false, false, nil`。路由层误报“no available account supporting this model and endpoint”，最终变成 `NO_AVAILABLE_GROUP`（503），被作为服务故障记录。

原代码运行以下测试失败：

```sh
GOTOOLCHAIN=auto GOFLAGS=-p=2 go test -C backend ./internal/handler ./internal/server/routes   -run 'TestAPIKeyGroupProbePreservesGroupRPMReason|TestMultiGroupRoutingPreservesGroupRateLimitAndFallback' -count=1
```

Handler 的 at_limit 用例期望 GROUP_RPM_EXCEEDED，实际 nil。路由测试注入这一明确业务错误后，原实现立即终止，未继续后续分组。

## 修正

- 对分组 RPM 保留业务错误，并继续只读检查模型、端点和账号可调度性。无关分组不贡献限流原因。
- 路由记住可承接请求的受限分组，继续后续分组；后续可用则正常选择，否则返回 429 GROUP_RPM_EXCEEDED。
- 全局限制及技术故障保持原处理；不放宽模型权限，不修改线上 5 RPM，不增加上游调用或 RPM 计数。
- 通过真实鉴权、路由及运维错误中间件验证：返回 429，日志标记 IsBusinessLimited=true。保留可审计日志，不承诺所有错误列表完全不显示。

验证命令：

```sh
GOTOOLCHAIN=auto GOFLAGS=-p=2 go test -C backend ./internal/handler ./internal/server/routes ./internal/service   -run 'TestAPIKeyGroupProbe|TestMultiGroupRouting|TestMultiGroupRPM|TestAPIKeyGroupRoutingLimits' -count=1
```

本地修复尚未发布、打 tag 或部署。上线后仍需观察：真实容量不足继续返回 503；业务限流为 429。若用户需要更高吞吐，调整 RPM 是独立的运营配置决策。
