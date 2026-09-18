# 多分组密钥迁移到 0.2.5 与中国验收记录

验证日期：2026-09-18。用户指出旧基线版本倒退后，将多分组功能实际移植到已发布的 `v0.2.5-custom.1`，并原地升级已有独立验收站；没有仅修改版本字符串。

## 当前入口

- <http://127.0.0.1:18081/keys>，隧道保持运行，原中国验收账号密码可登录。
- 当前版本：`0.2.5-custom.2-candidate.multigroup.20260918t100444z`。
- 原 `18080` 站仍为 `0.2.5-custom.1`，未重建其应用、PostgreSQL 或 Redis。
- 当前实例保留独立数据库及网络隔离，真实上游调用仍需后续验收。

## 代码基线

- 工作区：`/Users/varrge/workspace/kwRedeem/sub2api-multigroup-025`。
- 分支：`feat/multi-group-keys-0.2.5`。
- 基线：`v0.2.5-custom.1` / `1123dae2cef3da472e56d57d5dbaf1de765fa2f4`。
- 原脏工作区 `sub2api-custom` 与已发布版本工作区 `sub2api-cn-group-buy` 均保留。
- 源码快照含该工作区 tracked/untracked 非忽略文件；本部署报告在构建之后补录。
- 尚未提交、推送或创建正式 tag/Release；本候选未运行 GitHub CI，不能视为正式发布完成。

移植保留 0.2.5 的订阅/拼团页面分离、月卡冻结与额度窗口、API Key 批量编辑和供应商筛选、MiniMax/OpenCode Go、分组模型白名单、Codex 固定账号模型发现、WebSocket 安全检查及 Grok 视频槽位管理。

新增迁移改为 `242_api_key_multi_groups.sql` / `243_batch_image_billing_snapshot.sql`，鉴权缓存版本为 26。242 可用于已执行旧 236/237 的候选库，重建触发器而保留已有绑定及任务快照。没有重跑旧月卡 234/235 迁移，没有删除新版模型配置列。

兼容回归覆盖：所有候选模型名先经过各组白名单；Grok 探测遵守新版团队/模型冷却与模型额度；免费 input_tokens 接口豁免利润门；旧确定性兜底重新校验已选目标组权限与模型策略；OpenAI/Codex 固定发现保留元数据、空结果及配置兜底；Gemini 单模型查询只从授权多组目录返回。

## 验证

- Go 1.27.0：完整 unit 57 包、integration 51 包通过，Docker PostgreSQL 18 及月卡/支付专用 DSN 实际运行。
- Node 20.20.2 / pnpm 9.15.9：frozen install、typecheck、lint、315 个文件 / 2363 项测试、生产构建通过。
- golangci-lint：0 issues；部署脚本检查及 diff 检查通过。
- 最后目录修正后重跑 handler/routes 的多组及 Gemini 模型专项检查，重新 lint 和 Linux embed 构建，通过。
- 独立代码审查发现的问题均经修正；另检查新版月卡冻结、批量异步计费、软删除对象结算、迁移重复应用，无重要剩余发现。
- 中国部署后：健康、版本、实际密码登录、有序多组选组、普通用户/管理员资格限制、调整组顺序保持共享总额度和窗口用量、目录合并去重均通过。
- 升级前后对所有旧密钥的 `group_ids`、sticky 模式、quota、quota_used、5h/1d/7d 用量及 expires_at 逐项比较，相同。
- 升级前记录数量：users 152 / accounts 57 / api_keys 220 / subscriptions 49 / groups 36（包含此前临时测试的软删除记录）。本次测试账号及密钥测试后已软删除。
- 迁移 242/243 已记录成功；应用首次切换约 6.2 秒 healthy，另执行重启检查。
- 隔离网络下价格下载、版本同步、订阅邮件失败为预期；无缺列、迁移失败或 fatal/panic。

检查日志前缀 `/tmp/sub2api-025-multigroup-`：`final-unit.log`、`final-integration.log`、`final-lint.log`、`frontend.log`、`frontend-build.log`、`final-catalog.log`。

## 制品与备份

| 项目 | 值 |
| --- | --- |
| 镜像 | `sub2api-custom:0.2.5-custom.2-candidate.multigroup.20260918t100444z` |
| 镜像 ID | `sha256:1335cf9842903ec97d676f63e63cc497bc3a0f4db2bec239793ebf9d36e0fdf9` |
| 应用容器 ID | `ea2bdae1f0e3f55294608b656a741b68160dc72b30c653841aedafaad050baf5` |
| 源码清单 SHA-256 | `16c7b6865b89caa69c755e5a43da1b69aee74a73f6ebdc54b5df06820fa3ff3a` |
| 源码归档 SHA-256 | `07c948767f19f8e1aeb1747642a3610d3051ae9dd4e40cd97403cb99fde1fe86` |
| 运行二进制 SHA-256 | `c821d3a2f4926e30dcec7d12d1b611e342da8ab0c96bf53587804c9db0f88613` |
| 切换时间 UTC | `2026-09-18T10:11:31Z` |

远端新制品与备份根目录：`/home/yinan/sub2api-staging/multigroup025-20260918T100444Z`。

- `artifacts/` 保存源码、Docker 构建上下文校验和、部署与接口检查报告。
- `backup/` 保存切换前 PostgreSQL 一致性 dump、Redis RDB、应用配置/数据、原容器 ID、密钥配置比较快照及校验清单；目录私密，不打印配置或提交。
- 精确旧应用镜像：`sub2api-multigroup-rollback:20260918t100444z`。
- dump 已通过 `pg_restore --list` 校验，大小 66,051,309 字节。

运行 Compose 仍在 `/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json`，包含私密配置。project 为 `sub2api-multigroup-cn`。仅用 `up -d --no-deps sub2api` 重建应用；该实例数据库、Redis 容器 ID 未变，未覆盖数据库。

当前应用 IP `172.26.0.4`，本地端口 `18081`。隧道 control socket 沿用 `/private/tmp/sub2api-cn-multigroup-20260918T082757Z.sock`。重连命令见旧 [部署记录](deployment-cn.md#隧道重连和停止)，连接目标未变。

如本候选出现问题，应先停止候选应用并检查是否已有待结算多组异步任务。旧 0.2.0 回滚镜像只作为应急点，不能当作正确长期版本；优先修复 0.2.5 候选。任何应用回滚均不得自动恢复数据库。日本生产未操作。
