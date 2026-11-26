# v0.2.25-custom.7

基于 v0.2.25-custom.6，修复 Seedance 视频任务查询与计费提示问题。

## 修复

- 兼容上游文档中的任务状态别名：`pending`、`in_progress`、`running`、`succeeded`、`success`、`done`、`error`、`cancelled`、`canceled`、`expired`。此前这些合法状态会被本站误判为 502。
- 失败终态允许携带 `error` 对象，不再误报为无效任务响应；未知状态仍拒绝。
- 支持上游文档中的 `state` 字段作为 `status` 的兜底。
- 只有 `succeeded` 标记完成并计费，失败、取消、过期不计费。
- 按秒计费缺少或填写不支持的分辨率时返回 400 并提示可用档位；只有真正缺少价格时才返回 503 `pricing_unavailable`。

## 验证

- 新增状态归一化表驱动测试，覆盖 16 种上游状态与 `state` 兜底。
- 新增终态错误对象、未知状态、缺分辨率 400 的回归测试。
- `go test -tags=unit ./internal/service ./internal/handler` 全量通过，`go build ./...` 通过。

首帧图生视频 400 仍待上游按原 request ID 确认具体拒绝字段，本版本未改变首帧转换结构。
