# Ultrafast 相关 PR 对比（2026-10-19）

检索官方仓库 `Wei-Shaw/sub2api` 的全部状态 PR，关键词包括 `ultrafast`、`ultra fast`、`ultrafast_multiplier`、`超快`、`service tier multiplier`；获取 #8203 的文件差异并核对计费、请求改写、表单和迁移代码。存在相关 PR，因此按本次要求先对比，没有重复创建或向他人发送评论。

## 后续处理：已合入本地自定义分支

用户确认后，已将 #8203 的来源提交合入 `feat/ultrafast-pricing`，同时保留独立倍率及未知模型默认 1 倍。迁移使用未占用的 254 编号，鉴权缓存升至 30，保留自定义多分组配置并按本地 schema 重新生成 Ent。迁移回填只在首次新增列时执行，重复运行不会恢复旧强制 Fast 状态。评审中另修复了创建 API 的显式空档位优先级，以及 Messages → Chat Completions 路径遗漏的分组档位／全局策略应用，并补齐流式与非流式回归。免费 Ultrafast 只改变实际扣款，具体配置与组合示例见 [操作说明](../docs/custom-ultrafast-pricing.md)。此为本地集成，不表示官方 PR 已被上游合并；未部署生产。

以下对比保留检索时的两份实现状态。

## 最直接相关：#8203（检索时开放、未合并）

- [PR #8203：按分组强制 OpenAI 服务档位](https://github.com/Wei-Shaw/sub2api/pull/8203)
- 本次读取的 head：`7c064ede617d7e3e94a81b90af4835fae6f72efd`，49 个文件。
- 检查时官方 main：`8f2cbb53d9d1c40e1865b07ac460571f7d4c48de`，尚无本 PR 改动。

| 项目 | #8203 | 本地独立倍率实现 |
| --- | --- | --- |
| 任意 Ultrafast 倍率，如 1、3.5、6 | 无；只有免费开关 | 渠道／分组的模型价格卡可填写，独立于 Fast/Flex |
| GPT-6.1 Sol 默认 Ultrafast 价格 | 修正为标准价 6 倍 | 同样修正为 6 倍；显式自定义倍率优先 |
| GPT-5.6 Sol 默认 Ultrafast 价格 | 保留通用 2 倍 | 明确保留该模型及别名的 2 倍默认值 |
| 强制 Standard / Fast / Ultrafast | 新增分组档位下拉框，替换强制 Fast 布尔开关 | 保留既有转发策略，倍率不切换档位 |
| 对不支持模型的保护 | 不主动强制 Ultrafast，保留客户端原档位 | 无内置 Ultrafast 定价且未自定义倍率时，不因该标签加价，按 1 倍计费 |
| 客户端自行对无默认定价模型传 `ultrafast` | 计费函数仍回退通用 2 倍 | 默认不额外加价；第三方模型可明确配置倍率 |
| “免费 Ultrafast”的账务语义 | 仅把用户实际扣款改为标准档，保留档位原始 TotalCost | 填 1 是修改价格卡倍率，TotalCost 和实际扣款基数都会按此计算 |
| GPT-6.1 Sol 官方 API 模型目录 | 本 PR 未修改 | 补充 Ultrafast 声明；尊重已有上游声明，OAuth 不凭模型名授予能力 |
| 数据迁移 | 分组 `forced_service_tier`、`free_openai_ultrafast` | 渠道价格 `ultrafast_multiplier`；分组价格沿用 JSON 存储 |

“不支持模型时不强制”与“未知价格不自动加收”是两处不同逻辑。#8203 的 `openAIModelSupportsForcedUltrafast` 只控制是否注入档位，其 `serviceTierCostMultiplier` 仍把 Ultrafast 与 Fast 一起按通用 2 倍处理。

源码依据：

- [#8203 请求改写与模型判断](https://github.com/Wei-Shaw/sub2api/blob/7c064ede617d7e3e94a81b90af4835fae6f72efd/backend/internal/service/openai_gateway_request_body.go)
- [#8203 默认倍率与 GPT-6.1 Sol 修正](https://github.com/Wei-Shaw/sub2api/blob/7c064ede617d7e3e94a81b90af4835fae6f72efd/backend/internal/service/billing_service.go)
- [#8203 免费档位的用户扣款处理](https://github.com/Wei-Shaw/sub2api/blob/7c064ede617d7e3e94a81b90af4835fae6f72efd/backend/internal/service/openai_gateway_usage.go)

结论：#8203 与本地功能互补，不能用它替代独立倍率。合入时已保留 nullable 自定义倍率优先级，避免其无条件 `UltrafastMultiplier = 6` 覆盖运营者填写的数值；强制档位和免费补贴仍应与价格卡分开。自定义版本已有其他迁移，不能直接照搬该 PR 的迁移编号。

## 其他相关 PR

| PR | 检查时状态 | 关系 |
| --- | --- | --- |
| [#8165](https://github.com/Wei-Shaw/sub2api/pull/8165) | 开放，未合并 | 按描述增加账号能力探测、路由和分组能力白名单；解决账号是否真的拥有 Ultrafast 权益，不是独立定价设置 |
| [#8143](https://github.com/Wei-Shaw/sub2api/pull/8143) | 开放，未合并 | 按描述修复 OAuth 中转响应档位与最终转发档位不一致；不修改模型倍率 |
| [#6571](https://github.com/Wei-Shaw/sub2api/pull/6571) | 已合并 | 原始 Ultrafast 转发、目录、策略和通用 2 倍支持 |
| [#7730](https://github.com/Wei-Shaw/sub2api/pull/7730) | 已合并 | GPT-6.1 Sol 支持及 Astra Ultrafast 6 倍；未给 Sol 补齐 Ultrafast 6 倍 |

本次只验证本地计费规则、表单与静态 PR 差异，没有部署生产、调用付费模型或重算历史账单。
