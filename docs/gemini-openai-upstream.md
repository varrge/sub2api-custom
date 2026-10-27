# Gemini API Key 的 OpenAI 兼容上游

Gemini API Key 账号新增“上游协议”选项，账号平台、所属分组、月卡/企业计费与风险审计仍按 Gemini 处理。

| 选项 | 凭据值 | 上游接口 | 认证 |
|---|---|---|---|
| Gemini 原生（默认） | `native` 或缺省 | `/v1beta/models/{model}:generateContent` / `streamGenerateContent` | `x-goog-api-key` |
| OpenAI Responses | `responses` | `/v1/responses` | `Authorization: Bearer …` |
| OpenAI Chat Completions | `chat_completions` | `/v1/chat/completions` | `Authorization: Bearer …` |

配置字段为 `credentials.gemini_upstream_protocol`。只对 Gemini API Key 账号生效；OAuth 和 Vertex Service Account 保持原协议。创建、编辑和批量凭据更新会校验协议值；选择兼容协议必须显式提供 Base URL。

## 配置

在账号管理创建或编辑 Gemini API Key 账号，填写第三方上游 Base URL 和 API Key，选择其实际支持的协议，配置模型白名单/映射后执行文本测试。Base URL 可使用域名根路径或 `/v1`；不应填写 Gemini `/v1beta/models/...` 生成路径。

无需把模型改成 GPT 名称，也无需把账号迁出 Gemini 分组。账号测试和后台同步模型列表跟随所选协议；兼容协议的模型目录请求 `/v1/models`，对外 Gemini 模型目录转换为 `models[]` 格式。模型列表成功不能代替实际文本、工具和流式生成测试。

## 转换与计费

支持 Gemini 原生生成、Anthropic Messages、OpenAI Chat Completions、OpenAI Responses 客户端入口。文本生成的流式/非流式结果转换回客户端协议。保留模型映射、工具调用 ID 与结果配对、系统提示、常见采样参数、图片/PDF输入以及缓存和推理用量；缓存读取与推理 token 不重复计费。

工具调用参数在组成完整 JSON 后输出，不将不完整 JSON 作为 Gemini `functionCall.args`。响应解析使用有上限的逐事件读取；连接中断或缺失终止事件不会合成成功终止。读取器关闭时关闭原上游响应体。

OpenAI 客户端的推理档位、JSON 输出格式和工具选择约束会保留到兼容上游；无法表达的参数在发送前返回 400。固定正数 Gemini thinking budget 与 OpenAI reasoning effort 不等价，不做猜测换算。

流式生成中途失败会返回对应协议的错误事件，并保留上游已报告的用量参与正常结算；未报告用量时不会虚构 token。已输出部分内容的失败流不会自动重试生成。

`countTokens` 在兼容模式下使用本地估算，不向上游发送生成请求、不产生生成费用；不是第三方模型的精确 tokenizer 结果。

## 能力边界

兼容模式不代表第三方上游实现了 Gemini 的所有能力。以下请求在本地明确拒绝，避免静默丢失参数：

- Gemini 显式 `cachedContent`、安全策略、原生 Google 搜索/代码执行工具。
- Gemini 图片/音频生成配置、多候选生成、`topK`。
- 固定正数 thinking budget、原生 Gemini thought signature 历史。
- Responses 上游的 `stopSequences`（Chat Completions 可保留）。

兼容账号不参与 Gemini 显式缓存创建及 Gemini 原生批量生图选号；原生账号的这些能力不变。HTTP 上游错误保留给原有重试和账号错误策略处理；是否能跨账号重试仍受已输出内容、状态上下文和可用账号限制。

## 官方稳定版合并

本次基于官方 `v0.2.18`，包含 Go 1.27.3、前端依赖更新、流式断连结算和调度缓存修复。合并时保留了自定义月卡、企业账户、活动开箱、UF 定价及费用限额错误码。

账号配额结算冲突分两种处理：普通用量在账号删除后跳过不存在的账号计数；批量生图余额捕获继续在账号计数失败时回滚整笔事务。已受理批量任务仍可对软删除的原账号结算。原有三项 PostgreSQL 回归覆盖这两种边界。

升级前仍需确认官方 246 生图权限迁移及 Grok Fast 默认价卡变化。此功能不改变已配置的渠道/分组价格，也不自动将任何线上 Gemini 账号切换为兼容协议。

## 回归验证

协议回归覆盖两种兼容上游 × 四种客户端入口 × 流式/非流式，并检查缓存、推理 token 和模型映射。额外覆盖重复文本 delta、工具调用 ID、JSON Schema/工具选择/推理档位透传、无终止事件、失败流保留用量、缺失用量不虚构费用。完整 handler 测试确认已报告的失败流用量只产生一次结算和一条使用记录。

本次还运行后端全量单元测试、前端 3,629 项测试、前端 lint/typecheck/build、golangci-lint v2.14.0、嵌入前端的后端构建，以及隔离 PostgreSQL 的批量结算、月卡和活动回归。日志保存在本机 `/tmp/sub2api-0218-gemini-validation/`。数据库回归使用专用测试库；没有修改生产数据库。

上述模拟上游测试不替代第三方真实调用验收。本次不部署服务、不打 tag，也未使用 Krill 账号产生真实生成请求。
