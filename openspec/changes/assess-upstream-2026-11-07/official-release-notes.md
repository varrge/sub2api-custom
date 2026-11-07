# 官方 v0.2.20 发布核查

核查日期：2026-11-07（Asia/Shanghai）。本文件负责官方发布事实与适配注意事项；定制分支的冲突、编译和业务兼容结论见同目录 `integration-assessment.md`。仅调查，没有合并、发布或调用真实生图账号。

## 固定版本

| 项目 | 核查结果 |
| --- | --- |
| 最新稳定版 | `v0.2.20`，GitHub API 标记 `draft=false`、`prerelease=false` |
| 发布时间 | 2026-11-07 06:31:24 UTC，即北京时间 14:31:24 |
| Tag 指向的提交 | `dab829a4e9b7d1cc307189aea8b0319eb29e29d0` |
| 同时核查的 main | `6b4b37db43443b5e88e424fe3c69602cc3810dfb` |
| main 相对 tag | 仅多一条 `chore: sync VERSION to 0.2.20 [skip ci]`，没有额外功能修复 |
| 官方 v0.2.19 提交 | `dece26153621959ccd7635814f4b2d08304e681a` |
| 两个官方 tag 差异 | 112 个提交；459 个文件，29,482 行新增、1,457 行删除（含测试和生成文件） |

来源：[发布页](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.20)、[latest API](https://api.github.com/repos/Wei-Shaw/sub2api/releases/latest)、[tag commit API](https://api.github.com/repos/Wei-Shaw/sub2api/commits/v0.2.20)、[tag 到 main 比较](https://github.com/Wei-Shaw/sub2api/compare/v0.2.20...6b4b37db43443b5e88e424fe3c69602cc3810dfb)、[两版差异](https://github.com/Wei-Shaw/sub2api/compare/v0.2.19...v0.2.20)。本地通过 `git rev-parse 'v0.2.20^{commit}'`、`git diff --shortstat v0.2.19 v0.2.20` 复核。GitHub compare API 只返回前 300 个文件，不能据此误记总变更数。

注意：核查时网页检索器访问 `/releases/latest` 仍命中 v0.2.19 的旧缓存；直接 v0.2.20 页面与 GitHub API 一致。定制分支已经额外纳入部分上游提交，因此真正待合并增量不等于两个官方 tag 的差异数。

## 有价值的增量

- **生图后台能力**：按 1K/2K/4K 档位选账号并修正溢出选号时的会话粘性；GPT Image 2.6 原生图片路径保留 `complexity` 和编辑参数。适合与我们的新生图 UI 配合，但应分别核验 UI 参数、账号映射、上游能力和实际返回，不代表所有模型都兑现指定输出尺寸/格式。来源：[尺寸分流 PR](https://github.com/Wei-Shaw/sub2api/pull/8462)、[原生图片 PR](https://github.com/Wei-Shaw/sub2api/pull/8557)、[原生请求实现](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/backend/internal/service/openai_images_direct.go)。
- **高并发与慢连接治理**：入口缓冲预算、分阶段内存释放、逐次读写超时、有限后台任务、上游断开后 drain 窗口；修正 Bedrock 帧长度分配前校验和 Gemini/Antigravity 读取上限，并减少热路径 JSON 拷贝、重复解析与逐请求数据库查询。来源：[资源治理 PR](https://github.com/Wei-Shaw/sub2api/pull/8484)、[热路径优化 PR](https://github.com/Wei-Shaw/sub2api/pull/8518)、[入口运维文档](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/deploy/EDGE_SECURITY.md)。
- **协议与调度修复**：OpenAI 非流式 Responses 的 JSON 回包、空 instructions、WS 指纹和首 token 记录；Gemini 工具媒体、Antigravity 原生 Gemini 流式与 schema 转换；Kimi 并发 403 短冷却；Anthropic setup-token 每周用量。部分 OAuth passthrough 修复已经在定制分支，不能重复计算收益。来源：[发布说明](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.20)、[完整比较](https://github.com/Wei-Shaw/sub2api/compare/v0.2.19...v0.2.20)。
- **管理功能**：OAuth 管理 MCP、管理员密钥权限细分及显式开启的全权兼容模式、注册邮箱精确黑名单。MCP 默认关闭，需显式配置 `MCP_PUBLIC_URL`。这些可随升级纳入，但本身不是立即切换生产版的理由。来源：[MCP PR](https://github.com/Wei-Shaw/sub2api/pull/8389)、[管理员权限 PR](https://github.com/Wei-Shaw/sub2api/pull/8476)、[MCP 配置测试](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/backend/internal/config/mcp_config_test.go)。

## 需要适配与核验的行为

### 新限制与计费边界

**发布说明把三个新配置错误写在了 `security.*` 下；源码与部署文档使用 `gateway.*`。** 升级时必须以源码为准：

```yaml
gateway:
  request_body_memory_budget_bytes: 1073741824
  request_body_read_timeout_seconds: 120
  downstream_write_timeout_seconds: 120
```

预算超出时返回 503 和 `Retry-After`；单请求大小超限仍是 413。默认 1 GiB 是缓冲内存准入预算，不是进程 RSS 上限；长 SSE 持有的请求体仍占预算。120 秒是单次停滞读取/写入的限制，不是整次生成必须在 120 秒结束。对应项设为 0 可禁用，但应按实例容量决定配置。来源：[配置默认值](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/backend/internal/config/config.go#L2598)、[配置示例](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/deploy/config.example.yaml)、[运维说明](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/deploy/EDGE_SECURITY.md)。

异步生图需要独立保留请求体预算，官方本轮已补上 `DetachRequestBodyMemory`。这是新增预算机制的配套修复，不能把 release 中“异步生图全部 503”的文字理解为现有 v0.2.19 定制版已存在该故障。与我们原始分组绑定、请求生命周期和任务恢复逻辑合并时必须同时保留两方语义。来源：[修复提交](https://github.com/Wei-Shaw/sub2api/commit/0853d16036)、[任务上下文实现](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/backend/internal/handler/image_task_handler.go)。

客户端断开后等待上游最终 usage 不再无限持续：通常取 stream data interval timeout（默认 180 秒）；OpenAI 图片按 image timeout（默认 900 秒），Grok media 仍用一般 drain 时长。超时后较晚到达的用量可能无法进入结算。兼容会话缓存固定限制 16,384 条/32 MiB，会提前淘汰尚未过 TTL 的绑定。通知队列过载可能丢弃 best-effort 通知，已提交账务不受该通知丢弃策略影响。我们的视频长任务、共享额度和推送需要针对这些边界回归。来源：[运维说明](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/deploy/EDGE_SECURITY.md)。

GPT-6.1 Terra 的订阅请求取消 >272k 长上下文附加费，API Key 仍按现有倍率处理；兼容路径的 `minimal` effort 改为拒绝。这里记录的是 Sub2API 本版行为，不是独立核验模型供应商的商业定价。来源：[Terra PR](https://github.com/Wei-Shaw/sub2api/pull/8365)。

### 数据库和依赖

- 官方仅新增 `249_mcp_oauth.sql`：三张 MCP OAuth 表和索引，通过外键关联用户；没有删改现有业务表。仍需与定制迁移编号、执行器及回滚流程检查。来源：[迁移](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/backend/migrations/249_mcp_oauth.sql)。
- 前端 `vue-i18n` / `@intlify/message-compiler` 从 9 升到 11.4.16，属于 major 依赖升级；需跑我们新增页面、多语言完整性、类型和构建验证。来源：[package.json](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/frontend/package.json)、[升级提交](https://github.com/Wei-Shaw/sub2api/commit/d851a38477)。
- Go 要求仍为 1.27.3；新增 MCP SDK 1.4.1 等依赖，brotli 升至 1.2.7、compress 升至 1.20.2。上游提交说明这些升级修复安全扫描告警；本次没有据此推断所有当前部署均存在可利用漏洞。来源：[go.mod](https://github.com/Wei-Shaw/sub2api/blob/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/backend/go.mod)、[压缩库升级](https://github.com/Wei-Shaw/sub2api/commit/b6c0ca4256)、[MCP SDK 升级](https://github.com/Wei-Shaw/sub2api/commit/fb22c92126)。

## 发布质量与尚未闭环的信息

同一 tag SHA 的 main push [CI 42345517120](https://github.com/Wei-Shaw/sub2api/actions/runs/42345517120) 成功；但 tag push [CI 42346869771](https://github.com/Wei-Shaw/sub2api/actions/runs/42346869771) 的后端 [test job](https://github.com/Wei-Shaw/sub2api/actions/runs/42346869771/job/128326419333) 在 Unit tests 阶段退出码 2，后续 Integration tests 被跳过。该次前端与 lint 成功；[安全扫描](https://github.com/Wei-Shaw/sub2api/actions/runs/42346869736) 和 [发布构建](https://github.com/Wei-Shaw/sub2api/actions/runs/42346869739) 成功。公开 API 没给具体失败测试，日志下载需授权（返回 403），因此不能确定是 flaky/环境还是代码问题，也不能声称官方 CI 全绿。以上状态通过 [check-runs API](https://api.github.com/repos/Wei-Shaw/sub2api/commits/dab829a4e9b7d1cc307189aea8b0319eb29e29d0/check-runs) 和 job API 交叉确认。

发布后的 [#8573](https://github.com/Wei-Shaw/sub2api/issues/8573) 和 [#8574](https://github.com/Wei-Shaw/sub2api/issues/8574) 报告 GPT Image 2.6 GIF/SVG 和指定尺寸问题。前者贡献者核查称合成 HTTP 测试保留尺寸/格式，原 [#8557](https://github.com/Wei-Shaw/sub2api/pull/8557) 也披露上游可能拒绝 GIF、改返回 PNG 或忽略尺寸。它们是未完成真实环境归因的报告，不足以确认 v0.2.20 新引入网关回归；也不足以向用户承诺这些新格式可用。

建议纳入适配候选，固定 v0.2.20 tag，先完成定制语义合并和实际验证。主分支目前没有额外功能补丁值得另追；发布尚新、CI 尚有一次未解释失败，不应直接把“官方稳定版”当作我们定制版本的发布验收。

API 原始快照保存于本机 `/tmp/sub2api-official-20261107/`，只供本次复核；长期证据以以上固定提交和发布链接为准。
