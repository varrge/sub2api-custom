# 0.2.17-custom.3

基于 `v0.2.17-custom.2` 合并 `feat/ultrafast-pricing`，集成官方 [PR #8203](https://github.com/Wei-Shaw/sub2api/pull/8203) 的来源提交 `7c064ede617d7e3e94a81b90af4835fae6f72efd`，并保留自定义版的独立 Ultrafast 倍率、多分组路由和长上下文政策。

- 分组新增“跟随请求 / Standard / Fast / Ultrafast”档位选择；强制 Ultrafast 仅作用于已知支持的模型。
- 分组和渠道模型价格卡新增独立 Ultrafast 倍率，允许任意有限正数。留空时 GPT-6.1 Sol、GPT-6 Astra 默认 6 倍，GPT-5.6 Sol 默认 2 倍，无内置定价的模型默认 1 倍。
- “免费 Ultra Fast”默认关闭。开启后仅将用户实际扣款按 Standard 计算，保留档位总成本和账号成本统计；需要按自定义倍率收费时保持关闭。
- 补齐 Messages → Chat Completions 的强制档位与全局策略处理，覆盖流式和非流式请求；修复新 API 显式跟随请求与旧强制 Fast 字段同时提交时的优先级。

升级执行迁移 253、254，原强制 Fast 自动迁移为 `priority`；新免费开关不自动开启。鉴权缓存版本升至 30。迁移回放不会覆盖后续修改的档位，历史日志及已扣费用不重算。回退旧版时需核对保留的旧强制 Fast 列，完整说明见 [配置与回退说明](../docs/custom-ultrafast-pricing.md)。

Ultrafast 页面、倍率卡及档位策略的前端回归已加入发布 CI。此 tag 触发仓库既有 Release 工作流，所有必需 CI 检查通过后才发布制品；合并和 tag 不自动升级服务器。未调用付费上游模型进行验证。

发布前验证：后端 `go test -tags unit ./...` 全部通过；前端 430 个文件、3327 项测试通过，ESLint、类型检查与生产构建通过；真实 PostgreSQL 迁移回放、schema、鉴权投影和分组／渠道倍率回读通过；Go 1.27.2 对应的 golangci-lint 2.14.0 为 0 issues；嵌入前端的后端构建、发布工作流策略和 10 项发布矩阵测试通过。新增计费测试明确使用 `unit` 构建标签，与其共享夹具保持一致。
