# 官方 v0.2.20 合并适配评估

评估日期：2026-11-07（Asia/Shanghai）。结论：**值得做适配升级；当前试合并结果不具备直接发布条件。** 本轮仅核实发布、检查源码、执行 `git merge-tree` 和静态探测，没有修改集成分支、提交合并、推送或部署。

## 固定比较点

| 对象 | 固定版本 |
| --- | --- |
| 当前集成分支 `custom/0.2.19` | `005397cf11bf77f0e9f8f3ef2bd777f545255ba0`，已包含最新生图/视频工作台 |
| 最新官方稳定版 | [`v0.2.20`](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.20)，`dab829a4e9b7d1cc307189aea8b0319eb29e29d0` |
| 官方发布时间 | 2026-11-07 06:31:24 UTC / 北京时间 14:31:24；GitHub API `draft=false`、`prerelease=false` |
| 官方 main | `6b4b37db43443b5e88e424fe3c69602cc3810dfb`，仅比 tag 多一个 VERSION 同步提交 |
| 实际共同祖先 | `a3298b46ebe239fb0720b068a1eb3383955bdac2` |
| 试合并树 | `30ccdc46f63e7f1ddfa7b4e07b6ceb6853264019`，含未解决冲突，不能运行或发布 |

官方 v0.2.19 到 v0.2.20 有 112 个提交、459 个文件变化。我们的分支已经含 v0.2.19 后的一部分修复，因此本次真正待合入为 106 个提交、457 个文件；其中 131 个文件与定制改动重叠，31 个文件产生文本冲突。统计由固定 refs 的 `git rev-list`、`git diff --name-only` 和 `git merge-tree --write-tree` 得到。

在线发布及依赖核对见 [官方发布研究](official-release-notes.md)。官方 `/releases/latest` 网页缓存一度仍指向 v0.2.19，因此本次同时核对实时 API 和 v0.2.20 直接发布页。

## 合并收益

- **生图能力与选号**：GPT Image 2.6 原生图片链路、1K/2K/4K 账号分流和尺寸溢出后的会话绑定，与我们的生图工作台互补。具体输出格式仍取决于供应商支持，不能承诺 GIF/SVG。来源：[图片链路](https://github.com/Wei-Shaw/sub2api/commit/7f2876def)、[尺寸分流](https://github.com/Wei-Shaw/sub2api/commit/e452e41e4)、[粘性修复](https://github.com/Wei-Shaw/sub2api/commit/6b565e45a)。
- **资源与稳定性**：限制请求体内存及读取时间、后台通知工作量，减少重复解析；新内存预算配有异步生图独立 reservation，须整体适配。来源：[资源保护](https://github.com/Wei-Shaw/sub2api/commit/457602c8f)、[异步图片预算](https://github.com/Wei-Shaw/sub2api/commit/0853d1603)、[请求解析](https://github.com/Wei-Shaw/sub2api/commit/6c140b897)。这些不是对现有 v0.2.19 异步功能已经失效的判断。
- **兼容与依赖修复**：Gemini 流和工具媒体、异常密钥处理、长上下文订阅计费，以及压缩库和前端国际化依赖更新。来源：[发布说明](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.20)。
- 管理 MCP 是额外能力，当前无需靠它驱动升级；保留未配置 `MCP_PUBLIC_URL` 时不启用的默认行为即可。来源：[官方配置](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/deploy/config.example.yaml)。

## 已确认的阻塞和适配要求

### 1. 无文本冲突的文件也有编译阻塞

试合并树的 `backend/internal/service/api_key_auth_cache.go` 中，`APIKeyAuthGroupSnapshot.RequirePrivacySet` 在第 70 行和第 163 行各出现一次。我们之前已将字段加入结构体，上游在另一位置新增同名字段，Git 自动合并却无法判断 Go 字段冲突。

用 Go AST 扫描 63 个没有文本冲突、双方均改动的 Go 文件，确认这一个重复结构体字段。必须去重并保留分组快照的双向映射；仅清理 `<<<<<<<` 标记不能使代码具备可构建性。上游来源：[隐私开关快照优化](https://github.com/Wei-Shaw/sub2api/commit/9bb073d0d)。

### 2. 异步生图：内存预算与任务身份需同时保留

`image_task_handler.go` 同时修改 `newAsyncImageContext` 签名、取消逻辑和任务创建。上游新增独立请求体 reservation 及错误返回；我们的任务带 `GroupID`，并用 `RetainAccountRequest` 保持账号请求生命周期。

适配时必须组合两种资源释放，保留创建时 GroupID、原密钥/原分组轮询与恢复、无重复提交语义。不能在冲突处直接选一边。来源：[上游补丁](https://github.com/Wei-Shaw/sub2api/commit/0853d1603)、[当前 handler](../../../backend/internal/handler/image_task_handler.go)。

### 3. 生图/视频计费与共享额度通知

`openai_images.go` 的冲突涉及 `PricingAt` 和请求模型计费字段；`grok_media.go` 涉及 `PricingAt`、`VideoRateMultiplierOverride`；`gateway_usage_billing.go` 涉及配额落库、通知队列及 `personalUsageCost`。

采用上游有界后台任务时，仍须保留创建时价格、月卡周期、视频倍率和共享额度扣费后的个人实际支出，覆盖重复结算、恢复轮询、共享额度完全/部分承担的场景。否则可能编译通过但账单或低余额通知含义变化。来源：[当前图片 handler](../../../backend/internal/handler/openai_images.go)、[当前媒体 handler](../../../backend/internal/handler/grok_media.go)、[当前计费](../../../backend/internal/service/gateway_usage_billing.go)、[上游资源补丁](https://github.com/Wei-Shaw/sub2api/commit/457602c8f)。

### 4. 多分组鉴权、限流与账号池

上游 RPM 快照按单个分组保存，并新增 `UserGroupRPMOverrideGroupID` 区分“确认没有覆盖”和“尚未查询”。我们保存多个分组的 RPM map，还会动态切换分组。快照构造、反序列化和 `APIKey.SelectGroup` 要保持一致，缓存版本应高于当前的 32，不能退回上游的 27。

账号调度冲突则涉及上游尺寸分流与我们主/备用账号池、跨账号续接限制、并发 acquire-only 语义的组合。需明确尺寸筛选、池回退和粘性优先级，不能通过删去现有 wrapper 简化冲突。来源：[RPM 优化](https://github.com/Wei-Shaw/sub2api/commit/76221c9cb)、[尺寸分流](https://github.com/Wei-Shaw/sub2api/commit/e452e41e4)、[当前分组选择](../../../backend/internal/service/api_key.go)。

## UI、数据库和配置影响

- 生图页面、视频面板、imageStudio/videoStudio store 四个文件在试合并树中的 blob 与当前集成分支完全相同；上游不会直接覆盖刚完成的页面布局。全局导航、账号配置、密钥导出、国际化依赖仍变化，页面行为需重新验证。
- `vue-i18n` 从 9 升至 11，`@intlify/message-compiler` 同步升级；必须使用新版 lockfile 安装后运行类型检查、现有翻译/工作台测试和生产构建。来源：[依赖文件](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/frontend/package.json)。
- 官方只新增 `249_mcp_oauth.sql`，创建 3 张 MCP 表和索引，未修改既有业务表。它与我们的 `249_month_card_quota_adjustments.sql` 数字前缀相同但文件名不同；迁移器按完整文件名记录校验和，未发现名称碰撞，仍需临时 PostgreSQL 验证。来源：[官方 SQL](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/backend/migrations/249_mcp_oauth.sql)、[本地迁移器](../../../backend/internal/repository/migrations_runner.go)。
- 发布说明把请求体预算写在 `security.*` 下，但 tag 中实际代码和部署文档使用 `gateway.request_body_memory_budget_bytes`、`gateway.request_body_read_timeout_seconds`。应按代码配置。读取期限不等于整个图片任务的执行期限。来源：[官方部署说明](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/deploy/EDGE_SECURITY.md)。

## 建议执行范围

从当前 `005397cf1` 新建 v0.2.20 适配分支，合入固定 tag，不顺带追未发布功能。解决文本冲突及重复字段后依次验证：

1. 构建与生成代码；鉴权快照、切组 RPM、主/备用池、尺寸分流和 previous_response 粘性。
2. 异步生图提交/超预算拒绝/取消/恢复，原密钥查询，视频结算与价格快照，月卡/共享额度/通知。
3. 新依赖下前端类型、翻译、导航、密钥导出、图像和视频工作台测试及生产构建；后端现有单测与 lint。
4. 临时数据库迁移和账务集成测试，通过后再给发布结论。

本轮没有解决冲突、没有运行候选合并版本的全量测试或真实供应商调用。之前 v0.2.19 定制版的通过结果不能当作 v0.2.20 的验证结果。官方同 SHA 的 main CI 通过但 tag CI 有失败记录，详情和证据在配套发布研究中，未将其直接认定为代码回归。

## 可复查的本地证据

临时证据目录：`/tmp/sub2api-upstream-assessment-20261107/`。

- `metadata.json`：固定 SHA、合并树及完整 31 个冲突文件列表。
- `merge-tree.txt`：Git 试合并原始输出。
- `overlap-files.txt`、`upstream-files.txt`：重叠文件及上游增量文件清单。
- `probe.go`、`probe-paths.txt`、`probe-result.txt`：无文本冲突 Go 文件的 AST 重复字段探测。

以上仅为评估材料；目标分支保持 `005397cf1`，没有候选发布包。
