# Seedance 原生 API

支持火山方舟 Ark 的异步视频任务协议，无需把 `content[]` 转换成 OpenAI `messages` 或 Grok `prompt`。

## 配置

1. 创建 OpenAI 平台的 **API Key** 账号，填写 Ark API Key，Base URL 使用 `https://ark.cn-beijing.volces.com/api/v3`。兼容服务可填写自己的 `/api/v3` 或 `/v3` Base URL。
2. 在账号的端点能力中勾选 **Seedance (Ark)**。默认不启用，避免请求误调度到其他 OpenAI 账号。支持创建、编辑与批量编辑。
3. 将账号加入 OpenAI 分组并启用分组的「允许图片生成」媒体权限；合成分组也可路由到这些账号。
4. 配置模型映射，例如将公开模型名 `seedance-video` 映射到实际 `doubao-seedance-*` 模型或 `ep-*` 推理接入点；映射方式不变。
5. 按秒收费时，在管理端「分组管理 → 编辑分组 → 分组模型价格（可选覆盖）」添加客户端请求中 `model` 对应的模型名，选择「视频（按秒）」，填写默认视频每秒价格，或添加 `480p`、`720p`、`1080p` 分辨率层级价格，单位均为 **USD/秒**。也可在此分组**已关联的渠道**中配置同样的模型定价；未关联渠道中的同名价格不会生效。已有 `video_model_prices` 等显式分组视频价格仍可使用；自定义模型别名请通过上述「分组模型价格」或渠道规则配置，旧版视频价格面板会将部分别名合并为 Grok 模型族。

请求的模型及分辨率必须命中显式配置的价格，不能借用其他模型的价格，也不会回退到 Grok 默认价。价格留空表示未配置，填写 `0` 表示明确免费。已有分组或渠道的 token 定价继续兼容；切换为按秒收费时需将对应规则改为「视频（按秒）」。

## 调用

```bash
curl "$SUB2API_BASE_URL/api/v3/contents/generations/tasks" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "seedance-video",
    "content": [{"type": "text", "text": "海浪轻轻拍打沙滩"}],
    "duration": 5,
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": true
  }'

# 使用创建响应中的原生 id 查询，直至 succeeded / failed / cancelled 等终态。
curl "$SUB2API_BASE_URL/api/v3/contents/generations/tasks/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"

curl -X DELETE "$SUB2API_BASE_URL/api/v3/contents/generations/tasks/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"
```

亦支持 `/v3`、`/v1` 和无版本前缀别名。Ark SDK 的 Base URL 可改为 `$SUB2API_BASE_URL/api/v3`。文本、图片、视频、音频内容、角色及扩展参数原样传递，仅按账号配置改写模型名；响应保持上游原生格式。按秒计费请求须显式填写 `resolution`（`480p`、`720p` 或 `1080p`）；使用 `duration` 指定时长，`duration: -1` 或未指定时长时由上游确定实际时长。按秒模式不支持 `frames`，携带该字段会在调用上游前被拒绝；token 模式仍原样转发 `frames`。

## 任务与计费

- 查询和删除只能访问同一用户、同一 API Key、同一分组创建的任务，并始终使用原提交账号；不会转到其他账号查询。
- 创建时冻结计价模式、已配置的各分辨率单价及生效倍率，后续改价不影响此任务。按秒模式的费用为「冻结的分辨率每秒单价 × 实际时长 × 冻结倍率」，无需上游返回输出 token。
- 按秒任务首次查询到 `succeeded` 且有 `video_url` 后结算，优先使用完成响应的实际时长与分辨率；相应字段缺失时，才回退到创建时的正数 `duration` 与已定价 `resolution`。30 秒视频按 30 秒结算，不沿用 Grok 默认 8 秒或截短到 15 秒。自动时长任务必须取得正数实际时长；完成数据缺失或实际分辨率没有冻结价格时暂不结算并记录日志，后续查询可重试，不提前占用计费声明。
- 自动时长（`duration: -1` 或未指定时长）的在途额度预估按 30 秒上限保留，最终只按实际时长结算。兼容的 token 模式仍按完成响应的 `usage.completion_tokens` 计费。
- 重复查询由共享缓存声明和持久化用量去重共同保护。失败、排队及运行中的任务不计费。
- Redis 保存任务绑定及创建时的模型快照，默认 24 小时。需保留 Redis 状态并在有效期内查询完成结果。当前不会后台轮询；只使用回调而不查询的任务不会自动结算。
- 不开放上游的任务列表接口，防止共享账号的任务泄露给其他用户。删除遵循上游语义，不自动退款。
- 异步创建的上游错误不自动重试，以免重复创建付费任务。

协议依据：[火山官方 Go SDK](https://github.com/volcengine/volcengine-go-sdk/blob/master/service/arkruntime/model/content_generation.go)、[创建任务文档](https://www.volcengine.com/docs/82379/1520757)、[查询任务文档](https://www.volcengine.com/docs/82379/1521309)。
