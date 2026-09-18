# 多分组密钥适配官方 v0.2.6

日期：2026-09-18。

## 基线与范围

- 最新官方稳定版：[v0.2.6](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.6)，发布于 2026-09-18 09:58:27 UTC。
- 已发布自定义基线：`v0.2.5-custom.1` / `1123dae2cef3da472e56d57d5dbaf1de765fa2f4`。
- 已验收多组功能快照：`6a1323fb5`，从 `sub2api-multigroup-025` 原有改动完整复制后独立提交。
- 适配工作区：`/Users/varrge/workspace/kwRedeem/sub2api-multigroup-026`。
- 适配分支：`feat/multi-group-keys-0.2.6`，实际合入官方 v0.2.6，保留全部已发布自定义功能及多组功能。
- 原 0.2.0 脏工作区、0.2.5 功能工作区和已发布版本工作区均未修改。

本轮是兼容适配、合并评估与中国独立候选验证。仅本地提交，未推送、未创建正式 tag/Release，未操作日本生产。中国 18081 已升级到本次 0.2.6 候选；[port-025.md](port-025.md) 是上一版本的历史验证记录。

## 推荐理由及适配

v0.2.6 修复客户端取消后 Responses 会话归属丢失、Gemini 混合账号模型发现、OAuth 暂停调度时的刷新，以及多个支付和前端问题；更新 gRPC 与相关依赖，并优化分组用量查询。新 Codex ticket 收集/注入仍默认关闭，票据保留账号隔离、脱敏及编辑保护。

文本冲突处理保留双方意图：图片专用响应头超时与新增 harvest HTTP/1.1 独立连接策略并存；保留嵌套弹窗 Escape/滚动锁及官方唯一 ID 修复；供应商测试从实际 PROVIDERS 推导；依赖采用新版 crypto/net/gRPC，同时保留自定义已升级的 x/image v0.45.0。执行 go mod tidy 校验依赖。

独立审查确认必须补齐以下兼容问题：

1. 多组 Google 模型目录此前提前返回，遗漏 v0.2.6 新增的 Antigravity 混合模型发现。已增加 Google 专用目录：Gemini 组收入启用混合调度的 Antigravity Gemini 映射；直接 Antigravity/Composite 组遵循自身来源规则；排除 Claude 映射，保留原生 Gemini 合法别名、临时限流时的可见性、各组白名单和去重。
2. 客户端取消后的 Redis 绑定回归测试需包括新增的原组索引；使用新状态存储实例重新读取缓存，验证原组确实持久化，避免仅由进程内缓存掩盖失败。
3. 新版分组报表分两次查询读取水位和费用；并发清理历史用量可能混用快照。补充真实 PostgreSQL 并发回归，在同一只读 REPEATABLE READ 事务中读取，保留带参数的尾段索引查询。此问题影响管理员报表，不影响扣费和密钥额度的独立事务。

v0.2.6 无新增迁移；沿用自定义 238–241 和多组 242/243，鉴权缓存版本 26。保持密钥总额度及 5h/1d/7d 限制跨组选组共享，月卡/订阅资格和各组结算独立。

## 验证记录

独立审查发现的三项问题均已修复并复审，无剩余 Important/Critical 发现。

- Go 1.27.0：完整 unit 57 包、integration 51 包通过；真实 PostgreSQL 18 运行月卡、计费和支付 DSN 集成测试。单独 `go test -race ./internal/monthcard -count=1` 通过。
- golangci-lint 2.13.0：0 issues；Linux amd64 embed 构建通过，明确注入候选版本及已测试 commit。
- Node 20.20.2 / pnpm 9.15.9：frozen install、typecheck、lint、328 文件 / 2432 项测试、生产构建通过。
- 部署脚本完整检查通过（Apple container、Compose 安全、网关环境变量、运行资源及 Caddy 缓存）。
- govulncheck 调用路径扫描：0 个可达漏洞；依赖模块仍有 6 个未命中调用路径的公告，未宣称全部依赖无公告。
- 前端生产依赖审计通过仓库现有例外清单校验。

日志前缀：`/tmp/sub2api-026-`。

## 发布边界

建议合并此修正版 v0.2.6 适配；完整本地检查与独立审查已收口。本次 0.2.6 中国候选已完成隔离环境验收。正式发布仍需 GitHub CI 和固定 GHCR 镜像验证；真实上游调用受隔离网络限制，未进行联网消耗测试。候选构建必须明确注入 `0.2.6-custom.1-candidate` 版本；官方 tag 内 VERSION 文件仍为 0.2.5，不使用其默认值作为本次构建版本。

## 中国候选部署

- 访问入口：<http://127.0.0.1:18081/keys>，现有中国验收账号密码继续有效。
- 原 18080 仍为 `0.2.5-custom.1`，所有其他容器 ID 保持不变。
- 部署仅重建 `sub2api-multigroup-cn-sub2api-1`，使用 `up -d --no-deps`；候选 PostgreSQL/Redis 保持原容器，网络 `internal=true`。
- 当前版本：`0.2.6-custom.1-candidate.multigroup.20260918t103557z`。
- 代码合并 commit：`af3b2fa2b930e8fc29612b84dc77fece29be913a`，随后只追加验证文档。
- 镜像 ID：`sha256:ac957cdb9f267685cd181e6e6593a96103e537054b40360ad09a72dd3e84af25`。
- 应用容器 ID：`fe3c908d9a50feae35c355f15b4916bc3c68199c23b2c4c970ec43dd2f3d5e73`。
- 构建源码清单 SHA-256：`ddeb333722af9690be1b90816db18d7f89cef99bfb37d01f843d037a4bec685f`。
- 源码归档 SHA-256：`76353d10957222eb5ee7fcca577e41f127cb13d4480bd2d2b1b4fb5462cad5a3`。
- Linux embed 二进制 SHA-256：`8e1d1cd7b00bf9277dbeeddea012099128c74dd63321ba8e47a19b1ebdefb0d7`。
- 切换 UTC：`2026-09-18T10:41:16.300140+00:00`；约 8.3 秒 healthy；另重启后约 5.4 秒 healthy。
- 备份根目录：`/home/yinan/sub2api-staging/multigroup026-20260918T103557Z/backup`，PostgreSQL dump 66,031,460 字节，归档列表、Redis RDB、16 个文件校验和均通过。
- 本次精确回滚镜像：`sub2api-multigroup-rollback:20260918t103557z`，对应已验收 0.2.5 多组版本，不回滚至旧 0.2.0。
- 对所有旧密钥逐项比对 `group_ids`、sticky 模式、总额度、累计消费、5h/1d/7d 用量和过期时间，升级前后相同。
- 升级前 users153 / accounts57 / keys221 / subscriptions49 / groups36；检查后 users154 / keys222，其余不变，新增均为已软删除的临时测试用户/密钥。
- 21 项服务验收通过：真实密码登录、普通用户与管理员资格一致、多组创建/重排/缩减、共享额度保留、不合资格组拒绝、模型目录去重、分组报表接口、新 Codex tickets 默认关闭。
- 静态资源、本地隧道、版本、首页与 keys 页面检查通过；无 fatal/panic、迁移失败或缺列日志。

远端 `artifacts/` 保存源码、镜像上下文和结果记录。活动 Compose 仍在 `/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json`，含私密配置，不能打印或提交。候选 IP `172.26.0.4`；隧道控制 socket `/private/tmp/sub2api-cn-multigroup-20260918T082757Z.sock` 不变。

如需回滚，使用本次备份中的 `compose.rollback.json`，保持 `-p sub2api-multigroup-cn` 并仅 `up -d --no-deps sub2api`；不得自动恢复数据库。正式合并目标应保留 Fork 的 `custom/*` 分支流程，避免把自定义功能直接合入官方基线 `main`。
