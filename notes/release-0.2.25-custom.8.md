# v0.2.25-custom.8

基于 v0.2.25-custom.7，支持通过 `image_url` 传递首帧图片的 Videos JSON 上游。

## 背景

对美国服账号 149「🐁的SeedDance」直连上游实测：TTseadanco2.5-480 对 `first_frame`（字符串或对象）均返回 400「包含未支持的参数」，与客户两条失败请求完全一致；改用顶层 `image_url` 后任务成功，且用「画面内容自然运动」这类无主体提示词生成时，首帧就是所传图片。`reference_images` 也会受理，但图片不会作为首帧。

## 更新

- 新增账号凭证 `seedance_first_frame_field`，可选 `first_frame`（默认，保持现有上游兼容）或 `image_url`。
- 选择 `image_url` 时，`first_frame` 转成顶层 `image_url` 字符串；`last_frame` 在上游不支持，直接返回 400，不再向上游提交。
- 调度缓存保留该凭证字段，避免缓存路由时退回默认值。
- 账号新增／编辑页在 Videos JSON 协议下增加「首帧图片字段」选项。

## 升级

对使用 TT 系列模型的 Videos JSON 账号，在账号编辑页把「首帧图片字段」改为 `image_url`（或直接设置凭证）。默认值不变，其他上游无需调整。

验证：新增首帧字段映射与调度缓存投影单测；`go test -tags=unit ./internal/service ./internal/handler ./internal/repository`、`vue-tsc`、eslint、golangci-lint 与构建通过。
