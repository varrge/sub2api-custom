# 生图工作台：影策对照与改造方案

日期：2026-11-07。范围：源码分析与改造方案；不修改产品代码、不部署、不调用生图接口。

- 本项目基线：`f24cc29c4a20aa1ebde69933e0b32e72bb8e2017`。
- 影策基线：`cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828`，仓库版本 `v1.8.2`。
- 页面范围：影策 `web/src/pages/create/` 的快速创作页，对照我们 `ImageGenerationView.vue`；不把无限画布、短剧制作或电商 Agent 的功能混算成生图页能力。
- 方法：读取页面、状态、协议转换、任务 API 与相邻测试；结论中的“建议”是拟议行为，不代表当前已经实现。
- Gemini 分工：通过本机已有配置的 Gemini 模型接口（返回 `modelVersion: gemini-4-argon-high`）分析带行号的两边源码材料，负责页面交互与布局建议；主代理独立核对生成逻辑并形成本文。由于 CLI 与现有网关参数不兼容，本次采用直接接口提交材料，Gemini 没有独立运行浏览器或操作仓库。未改动用户的鉴权或网关配置。

## 结论

优先借鉴影策对生成过程的管理：一次生成有自己的输入快照、任务身份、部分成功结果和恢复入口。页面沿用我们已有的大图预览、底部输入区与历史侧栏，在结果上补齐继续创作的操作。

第一批收益最大的工作是：保住多图任务中已经成功的图片，补齐生成参数快照，再提供“复用参数”“用作参考图”。第二批让具备条件的 OpenAI/Grok 请求接入现有异步任务，实现刷新后查询原任务。模型能力和长期资源身份随后完善。

## 我们已有的能力，应当保留

| 已有能力 | 代码依据 | 方案边界 |
| --- | --- | --- |
| 会话、草稿、参考文件、结果按用户保存到 IndexedDB | [imageStudio.ts](../../../frontend/src/stores/imageStudio.ts)、[imageStudioStorage.ts](../../../frontend/src/stores/imageStudioStorage.ts) | 不另建一套 localStorage 历史；继续只存 API Key ID，不持久化密钥 |
| 用户切换隔离、异步回包归属检查、多标签页保存冲突检测 | `imageStudio.ts:116`、`:171`；`imageStudioStorage.ts:32` | 改为逐张回包或任务轮询后仍须保留这些检查 |
| 上传、拖入、粘贴参考图，自动进入图生图模式 | [ImageGenerationView.vue](../../../frontend/src/views/user/ImageGenerationView.vue)，`addReferenceImages`、`onPaste` | 参考图入口已经存在，需要改的是约束提示与结果回流 |
| 模型选择、尺寸联动、独立的分辨率和质量字段 | [imageGeneration.ts](../../../frontend/src/api/imageGeneration.ts)，`supportedImageResolutions`、`supportedImageAspectRatios`、`openAIImageSize` | 保留现有计费尺寸规则与协议适配，不引入上游旧版 quality/分辨率混用逻辑 |
| 大图预览、下载、会话历史、文生图/图生图筛选、手机历史抽屉 | `ImageGenerationView.vue` | 不把这些已有交互重复列为新增功能 |
| 同会话生成期间防止重复提交，切换会话后结果写回原会话 | `imageStudio.ts:157`；[imageStudio.spec.ts](../../../frontend/src/stores/__tests__/imageStudio.spec.ts) | 第一阶段保留同会话单次生成限制，暂不增加并行创作调度 |

当前状态恢复测试还明确验证：刷新中的请求标记为 interrupted，不自动重发。这一保护应保留；有任务 ID 时增加查询恢复，不用重新生成代替恢复。

## 可复用的生成逻辑

### 1. 多图部分成功：一张失败不应吞掉其余结果（P0）

**我们现在的行为。** `generateGeminiImages` 在函数内顺序请求多张图片，直到循环结束才返回数组（`imageGeneration.ts:192`）。如果第一张成功、第二张 HTTP 失败，函数会抛错；`imageStudio.ts:173` 等待整批返回后才写结果，所以本次已成功的第一张不会进入会话。这里是源码路径推演，未执行真实生图。

**影策的处理。** [create/index.tsx:989](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/pages/create/index.tsx#L989) 以批次任务收集结果，分别处理成功项、任务失败项、素材保存失败项；成功结果被合入消息，部分失败会显示成功和失败数量。

**最小改造。**

1. Gemini 顺序生成先保持不变，为每一项产生结果或失败状态；每得到一张便写入所属会话并保存。
2. 状态至少区分成功、确认失败、结果不确定、尚未提交。网络断开不能一律视为没有生成。
3. 页面显示“已完成 2/4”，保留成功图片，并单独显示异常项。“重新生成失败项”是新的生成请求；结果不确定项先提示核对，不能自动重发。
4. OpenAI/Grok 的 `n` 是一次请求返回多图，不要为了统一 UI 擅自拆成多个请求，避免改变并发和计费语义。

**落点。** `frontend/src/api/imageGeneration.ts` 的 Gemini 循环与返回合同，`frontend/src/stores/imageStudio.ts` 的写入流程，以及页面的生成状态区。

**验收。** 请求四张、第二张失败时第一张仍可预览下载；切换会话后成功项归属正确；切换账号后旧回包不进入新账号；空图片响应按单项异常处理；仅允许针对确认失败项发起明确的新请求。

### 2. 保存输入快照，让历史作品可以继续创作（P0）

**我们现在的行为。** `ImageStudioResult` 只记录模型、时间、比例、模式、prompt；没有质量、分辨率、原参考图集合、平台或任务 ID（`imageStudio.ts:19`）。会话里保存的是当前表单，后续编辑会改变它。生成函数内部还会归一化不支持的比例和分辨率，而结果元数据来自归一化前的 request。

**影策的处理。** [creation-types.ts](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/pages/create/creation-types.ts) 在创作记录上保留 settings、attachments、references、taskIds 和 clientOperationId；[create/index.tsx](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/pages/create/index.tsx#L1281) 使用任务上下文组织重试。

**最小改造。** 每次提交保存一份不可变的生成记录，结果通过 `runId` 关联该记录，不在每张结果上重复复制参考文件。快照应包含实际发送的模型、平台、prompt、尺寸/分辨率、质量、数量和参考图次序；保留 `apiKeyId`，不保存 key 明文。先在一个地方归一化请求，再让发送与记录共用同一份值。

页面增加两个明确操作。影策现有的 `createVariant/restoreMessageDraft` 也提供历史草稿回填入口（[create/index.tsx:1332](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/pages/create/index.tsx#L1332)），可以借鉴其交互目的：

- **复用参数**：把该记录填回输入区，用户仍需点击生成；原 Key 不可用、模型下线、参考图缺失时显示具体缺项，不自动改选另一个计费分组提交。
- **用作参考图**：把当前作品加入参考区，保留用户正在编辑的提示词。读取远端图片失败时提示失败；不能把未经验证的任意 URL 交给后端下载。先复用现有可信图片下载链路；需要代理时使用受鉴权、限大小和 SSRF 保护的资源接口。

老历史缺失的字段显示“未记录”，允许只复用已有内容，不编造原参数。这是本项目新增方案，不宣称影策快速创作页具有完全相同的两个按钮。

**验收。** 改写当前表单后，旧结果仍能恢复原来的参数；恢复参数不发出生成请求；实际降级的分辨率/比例与历史展示一致；已删除 Key 不被自动替换并消费；参考文件不持久化临时 blob URL。

### 3. 查询原任务与重新生成分开（P1）

**我们现在的行为。** 生图前端调用同步接口，没有保存后端任务 ID。刷新恢复时清空 pending 并标记 interrupted（`imageStudio.ts:135`）。已有后端异步接口，但前端尚未使用。

**影策的处理。** [creation-types.ts](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/pages/create/creation-types.ts) 区分 `syncInterrupted`、`errorKind` 和 `mediaRecoverable`；[task-center.ts:351](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/services/api/task-center.ts#L351) 将 retry 与 recover-media 分成不同 API；[generation-task-display.ts](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/lib/generation-task-display.ts) 单独展示提交结果不确定、生成中、保存未完成。

**我们可以直接利用的基础。**

```text
POST /v1/images/generations/async
POST /v1/images/edits/async
GET  /v1/images/tasks/{task_id}
```

[异步任务文档](../../../docs/ASYNC_IMAGE_TASKS.md) 与 [image_task_handler.go](../../../backend/internal/handler/image_task_handler.go) 明确：当前只支持 OpenAI/Grok 分组，依赖已开启且配置完整的对象存储；任务必须用提交时同一 API Key 查询，状态只有 processing/completed/failed。任务记录存在 TTL，执行仍由当前后端进程内 goroutine 承担；这不能描述成可跨后端重启恢复的持久队列。

**最小改造。**

1. 为前端提供当前 Key/分组是否可用 async 的只读能力信息，直接复用服务端 `Enabled()` 判断；当前不是已有前端合同，需要补上。不能靠先提交一次收费请求再猜能力。
2. 支持时提交异步请求，收到任务 ID 后立即持久化 `taskId + apiKeyId + runId`。刷新只 GET 原任务，遵循 `Retry-After`；暂停页面监听不等于取消后端任务。
3. 同步路径保留给 Gemini、Antigravity、Composite，以及未开启对象存储的部署。不能把 Composite 分组当成原生 OpenAI/Grok 分组直接开启 async。
4. POST 响应丢失且没有取得 taskId 时，记录“提交结果待确认”，不自动改走同步接口或重发。若要彻底解决该窗口，后续须增加服务端幂等键与任务查找能力，前端按钮锁不够。
5. 第一阶段只展示后端真实提供的 processing/completed/failed，加上前端 submitting/sync-interrupted。要显示“排队/生成/保存”三阶段，必须先扩展后端阶段字段，不能用计时器模拟进度。

**不能顺手承诺的能力。** 我们的 `ImageTaskService.Complete` 在对象存储上传失败时把任务标记 failed，没有持久保存待重试的媒体，也没有 recover-media API（[image_task.go:211](../../../backend/internal/service/image_task.go)）。因此，“仅重试保存、不重新生图”是另一个后端扩展，不能只加按钮。

**验收。** 刷新后继续展示同一个 taskId；查询断网不会发第二次 POST；切换账号停止旧轮询；Key 不可用、任务到期与未知结果有明确提示；后台关掉新任务入口后，已受理任务仍可查询；不把停止监听标为“取消成功”。

### 4. 用统一模型能力约束输入（P1）

**我们现在的行为。** 比例、分辨率、质量按平台和模型名称推导；参考图在页面固定为最多四张、每张 10 MB，输入框固定 4000 字符。超过参考数量时只取前几张，没有明确说明被忽略的文件。模型选择器支持手工填写，所以模型名称过滤并不意味着用户完全无法使用别名。

**影策的处理。** [model-capabilities.ts:74](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/lib/model-capabilities.ts#L74) 用 `ImageCapabilityConfig` 描述 prompt/参考图限制、size、resolution、quality、maxOutputs；[image-validation.ts](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/services/api/image-validation.ts) 在发送前检查参考图数量、大小与蒙版能力。

**最小改造。** 先把现有已验证规则汇入一个小的 `resolveImageCapabilities(platform, model)`，页面显示、参考图校验、请求归一化使用同一个结果；它仍可使用现有平台/模型规则，不必立即引入后端插件参数系统。未知自定义模型提示能力未确认，保守提供现有支持范围。

模型切换后如果超出参考图限制，保留文件并提示用户移除，不静默删除；上传超额文件说明接收与忽略数量。数字上限必须来自我们实际支持的渠道与后端校验，不照抄影策默认值。

**验收。** 模型切换后表单、发送值和快照一致；超额参考图得到明确提示；质量和分辨率继续独立；GPT Image 的像素限制、尺寸对齐及现有计费档位测试继续通过。

### 5. 作品身份与访问 URL 分开（P2）

**我们现在的行为。** `GeneratedImage` 只有 `src` 与 `revisedPrompt`。Base64 图片保存到 IndexedDB 后可以保留本地内容；远端 URL 则可能失效。已有 S3 上传服务返回公开 URL 或一次签名 URL，任务查询返回的是保存过的结果，并不自动续签（[image_storage_s3.go](../../../backend/internal/repository/image_storage_s3.go)）。

**影策的处理。** [creation-result-urls.ts](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/pages/create/creation-result-urls.ts) 把稳定的 storageKey 和短期展示 URL 分开，展示键不随签名更新而变化。

**最小改造方向。** 若要支持长期作品库，服务端返回受用户归属约束的资源 ID，并提供重新获取访问地址的接口；前端以资源 ID 作身份，URL 只是当前访问地址。现有 24 小时任务缓存不能充当长期作品库，需要另行明确保留期和清理策略。这个工作安排在页面与任务恢复之后。

## 页面方案的约束

Gemini 提出的同批图片对比、参数回填、历史搜索和参考图预览适合当前页面。下面是经过主代理核对、收窄后的布局方案。

桌面继续保留主预览区、底部创作输入和历史侧栏。第一阶段主区增加本次生成的结果缩略图条，复用生成记录的 `runId` 归组，暂不同时实现复杂网格切换；选中作品的工具栏提供预览、下载、复用参数、用作参考图。历史卡片显示一行 prompt 摘要，便于辨认作品。

```text
桌面
┌─────────────────────────────┬────────────────────┐
│ 图片 / 视频                 │ 历史               │
├─────────────────────────────┼────────────────────┤
│ 模型、参数摘要、图片操作    │ 搜索、会话选择     │
│                             │ 新建、重命名       │
│         当前大图            ├────────────────────┤
│                             │ 模式筛选           │
│ 同批结果  [1] [2] [3] [4]   │ 缩略图 + prompt    │
│ 本轮状态 / 恢复操作         │ 摘要 + 时间        │
├─────────────────────────────┤                    │
│ 参考图条 → 提示词 → 参数    │                    │
│                      [生成] │                    │
└─────────────────────────────┴────────────────────┘

手机：单列大图 → 同批缩略图条 → 本轮状态 → 输入区；
历史沿用抽屉，模型和尺寸按需展开。
```

手机保留单列，历史使用现有抽屉；参考图和本次结果缩略图横向排列，模型与尺寸面板按需展开。图片操作必须可点击发现，不依赖 hover。已有图片在新一轮生成时仍可查看，新任务状态放在独立卡片，避免整个预览区被骨架屏替换。

后续两项低耦合增强：

- **历史搜索与重命名**：参考 [creation-workspace-history.ts:83](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/pages/create/creation-workspace-history.tsx#L83) 的标题编辑与检索。我们先搜索当前会话的 `prompt/model`，并提供可见的编辑名称按钮；不把双击作为唯一入口，也不为少量本地历史引入搜索服务。
- **参考图预览与次序**：缩略图可放大，提供左移/右移操作和当前序号；上传、显示顺序与协议中的图片顺序一致。若后续加入 `@图片`，必须使用稳定 ID 并在发送前编译为实际图片次序，不能仅在输入框插入未经处理的 token。影策的 [expandCreationPrompt](https://github.com/ddcat-ai/open-ai-canvas/blob/cd2512eb9ba0aa7fa97ac7a72cb0d97c44c36828/web/src/pages/create/creation-references.ts#L99) 就在前端展开对应关系，不需要为了借鉴这个逻辑先迁入其后端技能系统。

对 Gemini 原始建议的调整：不照搬其“预留 saving 阶段”的建议，因为我们的 async API 尚未提供该阶段；不只依靠超时弹窗解决恢复问题，而是优先查询已有 taskId；费用预估暂缓的原因是尚未统一准确计价口径，并非用户使用 API Key 就无需展示费用。

按钮语义必须一致：

| 操作 | 是否发起新的生成 | 使用条件 |
| --- | --- | --- |
| 复用参数 | 否，只填表 | 有完整或部分历史快照 |
| 用作参考图 | 否，只加入参考区 | 原图内容可读且符合当前模型限制 |
| 重新获取结果 | 否，查询原任务或资源 | 有可查询的 taskId 或资源 ID |
| 重新生成 | 是 | 用户明确操作，重新验证权限和参数 |
| 重试保存 | 否，但需要后端支持 | 当前接口不提供，第一阶段不展示 |

## 落地顺序与交付边界

| 阶段 | 交付内容 | 主要改动位置 | 完成标准 |
| --- | --- | --- | --- |
| P0-A | Gemini 多图逐项保存、部分成功显示 | `api/imageGeneration.ts`、`stores/imageStudio.ts`、`ImageGenerationView.vue` | 中途失败不丢已成功图片；确认失败与结果不确定分开 |
| P0-B | 生成快照、同批结果导航、复用参数、结果回填参考图 | 同上；`imageStudioStorage.ts`、中英文文案 | 历史参数不被新表单覆盖；回填不触发生图；旧历史可读 |
| P1-A | 接上有条件的 OpenAI/Grok 异步任务恢复 | 上述前端文件、后端能力查询入口、异步接口文档 | 刷新继续查原任务，不自动重发；不支持的平台保持原能力边界 |
| P1-B | 统一模型能力、参考图限制提示 | `api/imageGeneration.ts` 或相邻纯函数文件、页面、文案 | 页面与实际请求一致；保留现有尺寸计费语义 |
| P1-C | 参考图预览/调序、历史搜索与可见重命名入口 | `ImageGenerationView.vue`、`imageStudio.ts`、文案 | 上传顺序与发送顺序相同；历史搜索不修改记录；键盘可完成重命名 |
| P2 | 稳定资源 ID/续签，必要时再做保存恢复 | `image_task.go`、`image_storage.go`、存储实现与资源访问 API | 到期 URL 可更新，跨用户隔离，保存失败有真实恢复材料 |

每阶段在现有 `imageGeneration.spec.ts`、`imageStudio.spec.ts`、`ImageGenerationView.spec.ts` 上补对应行为用例；异步扩展补后端归属、能力关闭、查询中断测试。布局阶段再做真实页面的桌面/手机、键盘、明暗主题验证。

本轮不建议迁入 React/Ant Design 组件、无限画布引擎、Agent 审批系统、独立素材库或完整模型协议插件平台。我们现有 Vue 页面和网关能够承接上述改造；只有明确需要新业务能力时才扩大范围。

费用预估也不放进第一阶段：本项目已有模型广场定价展示，但图片输入/输出 Token 单价不等于每张固定价，还涉及分组、用户、时段和计费档位。需要复用服务端真实计价口径后才能给生成按钮显示可信报价。

## 验证记录

已阅读本项目与固定版本上游源码、接口文档和相邻测试；Gemini 已返回完整页面分析（正常结束 `STOP`），其建议已由主代理对照代码取舍。文档内本地来源链接均已检查存在，新增文件位于现有 OpenSpec 变更文档目录，便于后续审阅。

未启动任一项目、未运行测试或构建、未调用生图接口；文档中的用户收益和布局为改造提案，不能视为运行验收结果。本轮仅新增本文，产品代码未改动。
