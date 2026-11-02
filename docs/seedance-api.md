# Seedance 视频 API

对外使用火山方舟 Ark 的异步视频任务协议。账号可选择 Ark 原生上游或 Videos JSON 上游，客户端仍使用相同的任务接口。

## 配置

1. 创建 OpenAI 平台的 **API Key** 账号，填写 Ark API Key，Base URL 使用 `https://ark.cn-beijing.volces.com/api/v3`。兼容服务可填写自己的 `/api/v3` 或 `/v3` Base URL。
2. 在账号的端点能力中勾选 **Seedance**，将「视频上游协议」设为 **Ark 原生（/api/v3）**。协议留空时仍默认 Ark；Seedance 能力默认不启用。支持创建、编辑与批量编辑，批量编辑默认保持已有协议。
3. 将账号加入 OpenAI 分组并启用分组的「允许图片生成」媒体权限；合成分组也可路由到这些账号。
4. 配置模型映射，例如将公开模型名 `seedance-video` 映射到实际 `doubao-seedance-*` 模型或 `ep-*` 推理接入点；映射方式不变。
5. 按秒收费时，在管理端「分组管理 → 编辑分组 → 分组模型价格（可选覆盖）」添加客户端请求中 `model` 对应的模型名，选择「视频（按秒）」，填写默认视频每秒价格，或添加 `480p`、`720p`、`1080p` 分辨率层级价格，单位均为 **USD/秒**。也可在此分组**已关联的渠道**中配置同样的模型定价；未关联渠道中的同名价格不会生效。已有 `video_model_prices` 等显式分组视频价格仍可使用；自定义模型别名请通过上述「分组模型价格」或渠道规则配置，旧版视频价格面板会将部分别名合并为 Grok 模型族。

请求的模型及分辨率必须命中显式配置的价格，不能借用其他模型的价格，也不会回退到 Grok 默认价。价格留空表示未配置，填写 `0` 表示明确免费。已有分组或渠道的 token 定价继续兼容；切换为按秒收费时需将对应规则改为「视频（按秒）」。

## Ark 原生调用

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

## Videos JSON 上游（吱吱鼠AI）

此适配依据提供的《对接文档_-_吱吱鼠AI.md》。它要求上游提供 `POST /v1/videos`、`GET /v1/videos/{id}` 和 `GET /v1/videos/{id}/content`，与 OpenAI multipart Videos 或 Grok `/videos/generations` 不是同一协议。

1. 编辑对应 OpenAI API Key 账号，Base URL 填写 `https://sub.zzshu.cc` 或 `https://sub.zzshu.cc/v1`，使用该上游的 API Key。
2. 勾选 **Seedance**，将「视频上游协议」设为 **Videos JSON（/v1/videos）**。对应凭据字段为 `seedance_upstream_protocol: "videos_json"`。
3. 保留公开模型到上游实际模型的映射。例如 `seedance-2.0-720p` → `SD2.0-9图满血版-720P`，名称以账号当前可用模型为准。
4. 给公开模型配置「视频（按秒）」价格及其支持的分辨率。Videos JSON 没有约定 token 用量，只有 token 定价时会在提交上游前返回 `503 pricing_unavailable`。当前按秒定价支持 `480p`、`720p`、`1080p`，不支持 `4K`。

客户端调用示例：

```bash
curl "$SUB2API_BASE_URL/api/v3/contents/generations/tasks" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "seedance-2.0-720p",
    "content": [
      {"type": "text", "text": "镜头缓缓推进，海浪拍打沙滩"},
      {"type": "image_url", "role": "first_frame", "image_url": {"url": "https://example.com/public-first.png"}}
    ],
    "duration": 5,
    "resolution": "720p",
    "ratio": "16:9"
  }'

curl "$SUB2API_BASE_URL/api/v3/contents/generations/tasks/$TASK_ID" \
  -H "Authorization: Bearer $SUB2API_KEY"

curl "$SUB2API_BASE_URL/api/v3/contents/generations/tasks/$TASK_ID/content" \
  -H "Authorization: Bearer $SUB2API_KEY" \
  -o video.mp4
```

- 网关将 `content[]` 文本转换为 `prompt`，图片、视频和音频转换为上游对应素材字段。除 `model`、`content`、`duration`、`ratio`、`resolution` 外的 Ark 扩展字段会返回 `400`，避免参数被静默丢弃。
- 图片 `role` 支持 `first_frame`、`last_frame`、`reference_image`，视频和音频分别使用 `reference_video`、`reference_audio`。未指定角色的素材作为参考素材。尾帧需要首帧；首尾帧不能与参考素材混用。参考图最多 30 张，视频、音频各最多 10 段。
- 上游素材必须是公网 HTTP(S) 文件直链。视频页面支持把图片拖入文本框、粘贴或点击左下角上传；网关会将上传文件转存到已有图片对象存储后，再把生成的直链或签名链接交给 Videos JSON 上游。管理员须先在「备份 → 图片存储」配置并开启图片存储，存储端点或其签名链接需能被上游访问；可以复用现有 S3 备份凭据。未配置或转存失败时返回 `503 media_storage_unavailable`，不会提交付费视频任务。折叠的「使用图片链接」仍可直接填写公网 URL，无需转存。
- 上传参考图支持 PNG、JPG、WebP，每张最大 10 MB、2500 万像素，同时受配置中的图片存储大小上限约束。所有图片通过格式、尺寸和请求模式校验后才开始转存。对象位于图片前缀下的 `references/`，保留和过期清理沿用对象存储的生命周期策略；签名链接期限沿用图片存储配置。Ark 和原有生图接口仍沿用自身上传方式。
- 视频页面会识别模型名中的固定分辨率，例如 `seedance-2.0-720p` 自动选择 720p 并禁用分辨率选择；切换模型或恢复旧会话都会重新校准。无固定分辨率的模型仍可选择支持的档位。4K 模型显示固定 4K 并禁止提交，直至其计价得到支持。
- 上游 `processing` 映射为 `running`、`completed` 映射为 `succeeded`。完成时不要求上游返回视频 URL；网关返回自己的受保护 `/content` 地址。页面会使用创建任务的 API Key 获取视频并播放，不把密钥放入 URL。
- 下载支持 Range。跳转须通过出站地址校验；跨域下载不会携带上游凭据或账号自定义请求头，签名路径和查询参数保留原样。启用出站域名白名单时，也需允许上游使用的 CDN 域名。
- 首次查询或下载确认 `completed` 时，按创建时冻结的价格、倍率和完成时实际指标结算。同一任务查询、下载共享去重；确认完成后的下载失败或客户端断开也不会跳过结算。失败任务不结算。
- 账号协议记录在创建任务快照中，改协议不会改变已有任务的查询目标。旧快照没有协议字段时按 Ark 处理。
- 不支持删除 Videos JSON 任务，返回 `405`。创建请求不自动重试，避免重复生成。页面对 Seedance 每 30 秒查询一次，约 10 分钟后暂停并允许继续查询，不重新创建任务。
