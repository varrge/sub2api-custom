# Ultrafast 档位与独立倍率

渠道管理 → 编辑渠道 → OpenAI → 模型定价，或分组管理 → 编辑分组 → 模型定价，展开模型价格卡可设置 **Ultrafast 倍率**。

| 设置 | 计费方式 |
| --- | --- |
| 留空 / API `null` | GPT-6.1 Sol、GPT-6 Astra 为 6 倍，GPT-5.6 Sol 为 2 倍；无内置 Ultrafast 定价的模型为 1 倍，不额外加价 |
| `1` | Ultrafast 按标准档价格计费 |
| `3.5` 等正数 | 标准档各项 token 单价乘以该倍率 |

该配置独立于 Fast、Flex，覆盖模型默认值；分组价格卡优先于渠道价格卡。输入、输出、缓存写入、缓存读取均使用同一个 Ultrafast 倍率，之后仍按原有规则应用分组／用户等计费倍率。允许小于 1 的正数，不接受 0、负数或非有限值。

普通、Fast、Flex 请求不使用 Ultrafast 倍率。模型没有内置 Ultrafast 定价时，不再仅凭 `ultrafast` 标签套用旧通用 2 倍；如第三方上游支持该档位，可明确配置独立倍率。此规则只决定计费，不代表上游支持该档位，也不会将普通请求升级。API Key 上游明确降为标准档时继续按标准档计费；OAuth 的响应档位解释保持原有规则。

长上下文附加费与服务档位倍率分开计算。采用渠道定价时，已有“0 到无限、各项区间倍率为 1”的渠道配置仍保持上下文平价，Ultrafast 倍率照常生效。匹配的分组价格卡整体优先于渠道卡，并沿用分组长上下文预设，不继承渠道区间。GPT-6.1 Sol 订阅账号的模型长上下文豁免保持原有行为。

已合入 [官方 PR #8203](https://github.com/Wei-Shaw/sub2api/pull/8203) 的分组服务档位与免费 Ultrafast 功能（来源提交 `7c064ede617d7e3e94a81b90af4835fae6f72efd`）。分组管理 → 编辑分组 → OpenAI 服务档位，可选“跟随请求 / Standard / Fast / Ultrafast”。选择 Ultrafast 时，仅对已知支持的 GPT-6 Astra、GPT-6.1 Sol、GPT-5.6 Sol 及已支持别名强制档位；其他模型保留客户端原档位。HTTP 和 WebSocket 均执行此规则，之后仍受全局 Fast 策略的允许／过滤／阻止控制。倍率设置本身不改变请求档位。

“免费 OpenAI Ultrafast”与“免费 OpenAI Fast”各自独立。免费 Ultrafast 开启后，用户实际扣款按 Standard 重算，保留相同的分组／用户倍率、高峰及长上下文政策；日志档位、各项 token 成本、TotalCost 和账号成本统计不因此降为标准档。它不代表整次请求免费，也不免除适用的长上下文费用。

| 配置 | 日志 TotalCost | 用户 ActualCost（示例分组倍率 0.5） |
| --- | --- | --- |
| Ultrafast 倍率 3.5，免费关闭 | 标准总价 × 3.5 | 标准总价 × 3.5 × 0.5 |
| Ultrafast 倍率 3.5，免费开启 | 标准总价 × 3.5 | 标准总价 × 0.5 |
| Ultrafast 倍率 1，免费关闭 | 标准总价 | 标准总价 × 0.5 |

需要按自定义 Ultrafast 倍率实际收费时，关闭“免费 OpenAI Ultrafast”。需要运营补贴、同时保留档位价格记录时，开启免费开关。

官方 OpenAI API Key 账号的 GPT-6.1 Sol 模型目录会声明 Ultrafast 能力；OAuth 和第三方账号以其上游模型目录为准。显式的上游能力声明优先于本地默认值。

部署时程序自动执行 `253_channel_ultrafast_multiplier.sql`，为渠道价格表新增可空字段；`254_group_forced_service_tier.sql` 新增分组强制档位和免费 Ultrafast 列，原“强制 Fast”开启的分组迁移为 `priority`，其余跟随请求。新免费开关默认关闭。旧 API `force_openai_fast` 仍兼容，优先使用新字段 `forced_service_tier`。鉴权缓存版本升至 30，旧缓存失效后重建，保留自定义版多分组路由。未填写的旧配置保持空值；部署后新的 GPT-6.1 Sol Ultrafast 调用默认从原有通用 2 倍修正为 6 倍，无内置 Ultrafast 定价且未配置倍率的调用从通用 2 倍改为 1 倍。历史日志、已扣款项不会重算。回退旧程序可保留新增列；旧程序不会应用独立倍率或新档位／免费设置。旧 `force_openai_fast` 数据库列保留但不再同步写入，回退前应核对并按需把新 `priority` 配置映射回旧列，防止恢复陈旧的强制 Fast 状态。用旧版编辑分组／渠道价格卡也可能丢失新字段。

已验证：计费及长上下文回归、免费开关与任意倍率／分组倍率的实际扣款和账号统计、PostgreSQL 新建／更新／清空／批量读取及迁移回放、分组保存与多分组鉴权缓存、HTTP／WebSocket 档位策略及 Messages → Chat Completions 流式／非流式转发、前端组合保存回读与校验、前后端构建。未调用付费上游 API，未部署至生产服务器。
