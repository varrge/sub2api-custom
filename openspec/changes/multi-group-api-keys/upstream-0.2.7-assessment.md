# 官方 v0.2.7 合并评估

评估日期：2026-09-20。结论：**推荐适配后合并，不能直接解完文本冲突就上线。** 本文保留最初分析和隔离试合并结果。后续已获授权完成兼容合并及中国候选部署，见 [实现说明](upstream-0.2.7-integration.md) 和 [部署记录](upstream-0.2.7-deployment-cn.md)；尚未创建新 tag。

## 固定比较点与历史

- 自定义版本：`8af0cdf81d3f6a253a79843630a01be9ad7f8b95`，包含多分组密钥、模型允许/禁用、拼团月卡、条件定时测试。
- 官方 [v0.2.7](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.7)：`aea725f2ea644d5592d0bbb1d63b607efa7e200a`。
- 官方 v0.2.6：`49a39b6dc1abed30fd227611e8af1108bc427610`。
- 实际共同祖先：`efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`。

官方两个 tag 的历史并不连续：`git rev-list --left-right --count v0.2.6...v0.2.7` 为 `7 / 18`。0.2.6 独有的 7 个提交属于 Codex 票据功能分支。直接比较版本树有 117 个文件差异，但真正合入的上游增量为共同祖先到 0.2.7 的 **59 个文件**，其中 **11 个文件**与自定义改动重叠。

`git merge-tree --write-tree 8af0cdf81 v0.2.7` 得到树 `712498980a8c6a5e765997f11b313201045ddcbe`。模拟树完整保留 `openai_codex_ticket.go` 和对应设置，不能把版本树中的“删除”直接理解为正常合并会移除现有票据功能。

## 值得合并的收益

- Kimi 等 Coding Plan 配额耗尽的特定 403 按临时限流处理，避免累计后永久标记账号错误：[上游修复](https://github.com/Wei-Shaw/sub2api/commit/db8692d67)。
- DeepSeek Chat fallback 保留/补齐思考模式工具回合所需 `reasoning_content`：[上游修复](https://github.com/Wei-Shaw/sub2api/commit/bcc73f8d4)。
- Anthropic 工具参数根级 union schema 兼容：[上游修复](https://github.com/Wei-Shaw/sub2api/commit/a8a1a3a95)。
- Antigravity 原生 Gemini 裸模型按思考档位解析，兼容 Go/Python GenAI 的 SSE 心跳格式：[模型修复](https://github.com/Wei-Shaw/sub2api/commit/0f4d8acaa)、[流修复](https://github.com/Wei-Shaw/sub2api/commit/f79b8bf96)。
- 新增 Seedance 原生异步视频协议和插件宿主服务/状态桥：[视频 PR](https://github.com/Wei-Shaw/sub2api/pull/7247)、[插件 PR](https://github.com/Wei-Shaw/sub2api/pull/7351)。这些是新增能力，不是已有多分组/月卡功能的替代。

本次无新增 SQL 迁移，`go.mod/go.sum`、前端依赖及构建版本没有增量。官方流水线通过不能代替自定义合并后的验证。

## 文本冲突

| 文件 | 冲突原因 | 合并要求 |
| --- | --- | --- |
| `backend/internal/handler/grok_media.go` | 官方把 Grok 固定账号选择推广到 Seedance；我们增加原始账号失败后的退出保护 | 保留失败保护，采用支持平台参数的新选择函数 |
| `frontend/src/components/layout/AppHeader.vue` | 官方手机端显示模型广场入口；我们增加顶部快捷菜单及玻璃风格 | 保留自定义入口条件和测试标识，兼容官方移动端入口及无障碍属性 |

## 文本冲突之外的阻塞项

### 1. Seedance 计费调用无法编译，不能仅补参数

隔离归档解决上述两处文本冲突后，`internal/handler/grok_media.go:540` 的 `recordGrokMediaUsage` 调用缺少自定义的计费时点和倍率参数，后端 handler/routes 构建失败。

[官方 Seedance 结算函数](https://github.com/Wei-Shaw/sub2api/blob/aea725f2ea644d5592d0bbb1d63b607efa7e200a/backend/internal/handler/seedance.go#L40) 只返回探测结果，并直接申请 Redis 计费 claim。我们的 Grok 结算则保留创建时的分组、权益快照、月卡归属周期，并在月卡路径依靠持久幂等处理。因此新视频接口必须接入同样的账务约束，不能补两个参数后就视为完成适配。

### 2. Seedance 新建请求可能选错分组

`apiKeyGroupSupportsPath` 尚未识别 `/contents/generations/tasks`，会默认允许任意平台；预探测也没有要求 Seedance 专用账号能力。

隔离复现 `TestReview027SeedanceSkipsIncompatibleFirstGroup`：第一组 Anthropic、第二组 OpenAI 均提供同名模型，期望选择第二组，实际选择第一组。随后 Seedance handler 会拒绝该平台，而不会回到后续可用组。

需补齐接口平台过滤、Seedance capability 和媒体权限预探测。来源：[自定义路由](../../../backend/internal/server/routes/api_key_groups.go)、[自定义预探测](../../../backend/internal/service/api_key_group_gateway.go)。

### 3. Seedance 任务查询没有绑定原始分组

`ResolveAPIKeyPinnedGroup` 支持 Grok 的 `/videos/:request_id`，未识别 Seedance 的 `/contents/generations/tasks/:task_id` 和 `seedance:` 任务命名空间。

隔离复现 `TestReview027SeedanceTaskLookupUsesOriginalGroup` 返回 `nil, false, nil`，即跳过原任务分组解析。调整密钥分组顺序或权限后查询可能落到当前组，无法按创建时身份完成查询/结算。需明确接入 GET/DELETE 的原始任务归属与权限语义。来源：[自定义资源绑定](../../../backend/internal/handler/api_key_resource_group.go)。

## 本地验证及范围

- 所有试合并操作均在 `git archive` 生成的临时归档中；当前功能分支没有 merge commit。
- Vue 类型检查通过；前端相关 **66 项测试通过**，覆盖密钥编辑、允许/禁用切换、管理员多分组配置、拼团页面、顶部菜单、条件定时测试和翻译完整性。
- 后端 service、admin handler、middleware、monthcard、apicompat 的定向测试通过；未启动 PostgreSQL，因此不代表数据库集成验收。
- 初始后端整体定向命令因上述 Seedance 调用签名不匹配失败。
- 为继续诊断，仅在临时归档添加 `requestStart, nil` 作为编译占位；在此基础上，既有 handler/routes 定向测试通过，新增两条复现测试失败，确认路由缺口。该占位不是账务修复，临时归档不可部署。
- 拼团/月卡核心代码、模型允许/禁用实现及条件定时测试本体在模拟合并中未被修改。Kimi 新 403 分支有“账号限流”和“快照缺失时临时停调”两种结果，后者不会触发当前仅观察模型/账号限流的条件定时测试，适配时需按既定触发范围验收。
- 没有调用真实供应商、没有全量 CI、没有中国服务器验收，不能宣称已经具备发布条件。

## 建议的下一步

在新的 0.2.7 适配分支合并，逐项解决两处文本冲突及三项 Seedance 兼容问题。完成定向回归、全量 CI、月卡真实 PostgreSQL 验证，再部署中国候选版本验收。正式通过后版本为 `0.2.7-custom.1`，不移动现有 tag。
