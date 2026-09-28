# 官方 v0.2.9 更新调查（2026-09-28）

调查时间：北京时间 2026-09-28 19:39–19:45。来源为 Wei-Shaw/sub2api 官方 GitHub 的 Release、commit/compare、PR、Issue、Actions 与公开安全公告；Issue/PR 作者的复现和测试陈述不等于本地独立验证。本次只做研究，没有合并、发布或变更生产环境。

**建议：值得以固定的 v0.2.9 提交为目标准备定制版集成；在计费、模型权限和月卡回归通过前，不直接替换生产镜像。** 这是集中修复版本，无新增数据库迁移或依赖升级，收益明确；但图片计费的实际金额会变化，且已知 prompt-cache 400 尚未解决。当前 main 并无额外业务修复，不必追浮动分支。

## 版本与验证事实

| 项目 | 已核实内容 | 来源 |
| --- | --- | --- |
| 当前定制评估基线 | `5e010e47f2b54a6ce6751811f8b3fee1eee292a0`；已包含官方 v0.2.8 `fd80b08c90b55edcad5b00171b53f08721d30da1` | 本次任务基线；[既有集成记录](official-v0.2.8-merge.md) |
| 最新正式版 | v0.2.9，非 prerelease；发布于北京时间 **09-28 11:08:59**（UTC `2026-09-28T03:08:59Z`） | [Release](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.9) |
| v0.2.9 固定提交 | `4c00df2e0183e2c70b7fa8ba45914205e36aad0c`，提交于北京时间 09-28 10:52:52 | [Commit](https://github.com/Wei-Shaw/sub2api/commit/4c00df2e0183e2c70b7fa8ba45914205e36aad0c) |
| 当前 main | `9a62841fd124d026cf3694fcf9b79e98addcdbdc`；比 tag 多 1 个提交，仅把 `backend/cmd/server/VERSION` 从 `0.2.8` 改为 `0.2.9` | [Compare](https://github.com/Wei-Shaw/sub2api/compare/v0.2.9...9a62841fd124d026cf3694fcf9b79e98addcdbdc) |
| 与 v0.2.8 差异 | 70 个提交（含分支提交与 merge），117 个文件，新增 3,133 行、删除 378 行；没有 `backend/migrations`、Ent schema、Go/npm 依赖清单或锁文件变化，也没有 release workflow 变化 | [固定提交差异](https://github.com/Wei-Shaw/sub2api/compare/fd80b08c90b55edcad5b00171b53f08721d30da1...4c00df2e0183e2c70b7fa8ba45914205e36aad0c) |
| 官方检查 | tag 对应的 CI、Security Scan、Release 全部 success；后端测试、前端、lint、shell 与 release helpers 各检查均完成 | [CI](https://github.com/Wei-Shaw/sub2api/actions/runs/36372069237)、[安全扫描](https://github.com/Wei-Shaw/sub2api/actions/runs/36372069240)、[发布](https://github.com/Wei-Shaw/sub2api/actions/runs/36372069259) |

tag 中 VERSION 仍为旧值是官方发布后回写版本号的流程结果；自定义发行仍应使用自己的明确版本号和 GHCR/CI 门禁，不照搬官方 main 的回写流程。截至调查，新版本只发布约 8.5 小时，线上观察期较短。

主评估同时完成的本地/线上只读核实：当前分支为 `custom/0.2.8`，美国线上实际二进制为 `0.2.8-custom.1`、同一 `5e010e47f2b54a6ce6751811f8b3fee1eee292a0` 提交，健康；旧 Docker 标签不代表正在运行的代码版本。`merge-tree` 模拟发现 31 个与定制修改重叠的文件，其中仅 5 个文本冲突：`VERSION`、`model_plaza_service.go`、`frontend/src/api/modelPlaza.ts`、`PlazaModelPricingTable.vue` 及其测试。后四项主要是定制版已有的视频独立倍率与上游实现重叠。文本冲突规模可控，但不能据此推断非冲突的计费/授权语义已正确；这些本地检查不属于官方发布保证。

## 值得吸收的修复

| 范围 | v0.2.9 的收益 | 原始依据 |
| --- | --- | --- |
| CC Switch | Codex 导入保留管理员配置的根地址，仅去掉尾斜杠；usage 脚本兼容带/不带 `/v1` 的 base URL，避免 `/v1/v1/usage` | [#7622](https://github.com/Wei-Shaw/sub2api/pull/7622)、[#7549](https://github.com/Wei-Shaw/sub2api/pull/7549) |
| Claude / OpenAI 兼容 | 保留 Anthropic structured-output beta、尊重禁用 thinking、GPT 5 以后代际按推理模型处理、空终态文本从流中恢复、保留 `content_block_start` 工具参数、补齐转换消息的 `type: message` | [#7638](https://github.com/Wei-Shaw/sub2api/pull/7638)、[#7538](https://github.com/Wei-Shaw/sub2api/pull/7538)、[#7568](https://github.com/Wei-Shaw/sub2api/pull/7568)、[#7569](https://github.com/Wei-Shaw/sub2api/pull/7569)、[#7570](https://github.com/Wei-Shaw/sub2api/pull/7570)、[#7635](https://github.com/Wei-Shaw/sub2api/pull/7635) |
| Codex 会话 / 调度 | 上下文 rollover 断开旧 websocket 响应链；保留多智能体 beta；探测模型不存在不再误判整个 Responses 端点不支持；额度暂停遵守已知未来 reset 时间；补回调度快照自动重置额度字段，并限制无额度/查询失败的重复查询 | [#7615](https://github.com/Wei-Shaw/sub2api/pull/7615)、[#7617](https://github.com/Wei-Shaw/sub2api/pull/7617)、[#7571](https://github.com/Wei-Shaw/sub2api/pull/7571)、[#7624](https://github.com/Wei-Shaw/sub2api/pull/7624)、[#7555](https://github.com/Wei-Shaw/sub2api/pull/7555) |
| 模型发现 / 映射 | OpenAI 透传账号补充默认模型列表，不再把同组其他账号的映射模型隐藏；识别 OpenRouter Opus 5.5 别名；OpenCode Zen 的 DeepSeek 回退补 reasoning 字段 | [#7526](https://github.com/Wei-Shaw/sub2api/pull/7526)、[#7597](https://github.com/Wei-Shaw/sub2api/pull/7597)、[#7562](https://github.com/Wei-Shaw/sub2api/pull/7562) |
| Antigravity / 可观测性 | 空 `MALFORMED_FUNCTION_CALL` 流触发 failover；转发 PDF；保留 schema 的 string const；客户端断开统一记录 499，减少虚假的上游 502/200 | [#7607](https://github.com/Wei-Shaw/sub2api/pull/7607)、[#7585](https://github.com/Wei-Shaw/sub2api/pull/7585)、[#7393](https://github.com/Wei-Shaw/sub2api/pull/7393)、[#7609](https://github.com/Wei-Shaw/sub2api/pull/7609) |

其中 Anthropic structured-output 修复已有报告者在真实 OAuth 账号上反馈：v0.2.8 返回 400，v0.2.9 对三个 Claude 模型的直接请求和 LiteLLM JSON schema 请求均通过。属于社区实测反馈，未在本环境重放。[原始反馈（09-28 14:22 北京时间）](https://github.com/Wei-Shaw/sub2api/issues/7633#issuecomment-5864591789)

## 计费、权限和迁移风险

Release 未列“破坏性变更”，但以下业务行为仍需明确验收：

1. **渠道图片价格留空将改变真实收费。** 原来空图片输出价被当成显式 0；现在留空继承 LiteLLM/内置目录图片价，目录没有才回退文本价。显式填写图片输出价 `0` 仍表示免费。主评估线上只读查询发现 24 条 channel token 定价中，9 条 `image_input_price` 为 NULL，9 条 `image_output_price` 为 NULL，其余 15 条图片输出价显式为 0；最近 7 天 `usage_logs` 中图片输入/输出 token 非零的记录共 0 条。因此存在受影响的配置，但未见近期图片 token 收费证据，不能说完全无风险。仍需核对月卡扣费及模型广场展示；不能仅凭“bug fix”判断对账单无影响。[#7573](https://github.com/Wei-Shaw/sub2api/pull/7573)、[定价代码](https://github.com/Wei-Shaw/sub2api/blob/4c00df2e0183e2c70b7fa8ba45914205e36aad0c/backend/internal/service/model_pricing_resolver.go)。本地查询摘要：工作区 `official-update-20260928/production-pricing-summary.txt`。
2. **账号统计成本的长上下文开关修正。** 目录价格回退路径使用账号长上下文开关，分组售价开关不决定上游成本；不应把这理解为重新改变用户端长上下文门控或新增迁移。统计利润/成本数字可能变化。[#7619](https://github.com/Wei-Shaw/sub2api/pull/7619)、[代码](https://github.com/Wei-Shaw/sub2api/blob/4c00df2e0183e2c70b7fa8ba45914205e36aad0c/backend/internal/service/account_stats_pricing.go)
3. **alpha search 失败不再按成功搜索收费；Free Fast 缺失定价仍保留零成本日志。** 这些改善计费与审计，但须保留定制月卡事务结算、跨卡分摊、余额补扣和欠费禁止新请求的链路。[#7572](https://github.com/Wei-Shaw/sub2api/pull/7572)、[#7613](https://github.com/Wei-Shaw/sub2api/pull/7613)
4. **模型白名单新增任意位置 `*`。** 匹配为大小写不敏感、首尾锚定，仅 `*` 是通配符，不能当成完整 shell glob。旧精确/尾通配规则继续支持；新规则的模型列表枚举并不覆盖所有无限模式交集。需与定制 Key 模型 ACL、多分组选择、`/v1/models` / Codex 目录联测，确保可见模型与实际调用授权一致。[#7595](https://github.com/Wei-Shaw/sub2api/pull/7595)、[实现及枚举限制注释](https://github.com/Wei-Shaw/sub2api/blob/4c00df2e0183e2c70b7fa8ba45914205e36aad0c/backend/internal/service/group_model_allowlist.go)
5. **模型广场视频独立倍率。** 后端响应、API 类型和前端展示一起改动；定制模型广场 UI 不能只接受某一侧，需核对独立视频倍率及已有思考等级倍率展示。[#7524](https://github.com/Wei-Shaw/sub2api/pull/7524)

本版没有新 SQL 迁移，不能据此省略定制支付/月卡/优惠券回归。现有迁移文件及校验值、冻结周期、优惠券作用域和并发事务语义均应保留。上述是集成验收要求，本研究未执行集成测试。

## 先前问题是否解决

| 问题 | 截至调查的状态 | 判断 |
| --- | --- | --- |
| GPT prompt-cache 400：[#7546](https://github.com/Wei-Shaw/sub2api/issues/7546) / [PR #7547](https://github.com/Wei-Shaw/sub2api/pull/7547) | Issue、PR 均 open，`merged_at=null`；v0.2.9 提交集合未包含该补丁 | **尚未由本次官方升级解决。** 报告者描述 v0.2.8 相对 v0.2.7 回归；拟议补丁在最终 GPT 模型映射后清除不支持的显式 cache hints，并保留 `prompt_cache_key`。PR 明示未做真实上游请求验证，不能宣称补丁已获线上证实。 |
| CC Switch Codex 根地址：[#7580](https://github.com/Wei-Shaw/sub2api/issues/7580) | #7622 于北京时间 09-28 08:48:38 合并，merge `f45126c8ec29914a9c266bbe7d0525f50f3cd7f2`；项目 OWNER 08:49:54 关联该 PR 并关闭问题；补丁位于 v0.2.9 | **官方已修复新导入的链接生成。** 已导入 CC Switch 的旧配置不由这段生成逻辑自动改写，应重新导入或修正现有端点。管理员显式配置的 `/v1` 会被保留，不会被删除。 |
| CC Switch usage 路径重复 | [#7549](https://github.com/Wei-Shaw/sub2api/pull/7549) 已合并并位于 v0.2.9 | **新生成的用量脚本已修复。** 与 Codex 根地址修复是两个不同位置的修改。 |

## 未解决报告与证据边界

- **Command Code Responses：** [#7640](https://github.com/Wei-Shaw/sub2api/issues/7640) 报告 v0.2.9 返回 `unknown field summary` / 缺失 `input.type`，但同一报告明确回退 v0.2.7 仍出错，强制 Chat Completions 可用。因此不能定性为 v0.2.9 新回归。[#7645](https://github.com/Wei-Shaw/sub2api/pull/7645) 的 client-tools 适配 PR 尚未合并，且其作者声明只完成 mock 路径测试，未原样重放原失败请求；不能把它当作该 Issue 全部错误的已验证修复。
- **GLM 转发：** [#7641](https://github.com/Wei-Shaw/sub2api/issues/7641) 报告 0.2.9 经 Aliyun model-router 转发失败而直连成功；尚无旧版对照、维护者确认或合并修复，属于需要场景复现的兼容性报告，不足以证明本版引入。
- **Antigravity 首内容超时：** [#7637](https://github.com/Wei-Shaw/sub2api/issues/7637) 描述大图片思考阶段无下游数据导致 30 秒客户端超时；[#7643](https://github.com/Wei-Shaw/sub2api/pull/7643) 尚 open，未进入 tag/main。拟议保活补丁发送首个 ping 后会提交 HTTP 响应，随后失败无法再透明换号，存在需要评估的行为取舍。
- **长连接账号状态 / Composite 路由：** [#7653](https://github.com/Wei-Shaw/sub2api/pull/7653) 报告 WS 后续回合不重新验证上游账号资格，[#7647](https://github.com/Wei-Shaw/sub2api/pull/7647) 报告 Composite alias 可能选到非所属账号；都有作者提供的测试说明，但均未合并，也未在本研究复现。与本项目权限/多分组相关，宜作为定向回归案例，不视为 v0.2.9 已解决或已证实新引入的漏洞。
- **“降智”、封号、套餐额度、模型权限**等报告没有形成足够的旧版/新版控制实验，不作为立即回退或跳过此版本的事实依据。[示例 #7598](https://github.com/Wei-Shaw/sub2api/issues/7598)、[#7563](https://github.com/Wei-Shaw/sub2api/issues/7563)

公开安全公告接口返回两项历史公告：CVE-2026-73079 / GHSA-vrxq-qm4h-6hgg（影响 v0.1.135–v0.1.168，v0.1.169 修复）与 CVE-2026-27812 / GHSA-vc2q-289v-74g3（v0.1.85 前受影响）。未发现官方将 v0.2.8 列为受影响、要求紧急升级 v0.2.9 的新公开公告；这不代表不存在未公开问题。[公告目录](https://github.com/Wei-Shaw/sub2api/security/advisories)、[路径穿越公告](https://github.com/Wei-Shaw/sub2api/security/advisories/GHSA-vrxq-qm4h-6hgg)、[重置密码公告](https://github.com/Wei-Shaw/sub2api/security/advisories/GHSA-vc2q-289v-74g3)

## 结论适用范围

官方资料支持“准备固定 v0.2.9 的受控集成”，不支持“所有兼容问题已解决”或“立即无条件换官方镜像”。最终是否合并，应结合定制分支实际重叠修改检查，以及月卡、模型授权、金额口径、CC Switch 和真实流式调用验证。未合并 PR 不应因出现在官方仓库就自动纳入更新包。
