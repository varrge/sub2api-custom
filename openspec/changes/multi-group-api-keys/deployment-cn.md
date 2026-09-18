> 本文记录首次 0.2.0 实施/部署。当前版本已迁移到 0.2.5，最新状态以 [port-025.md](port-025.md) 为准。

# 多分组密钥：中国独立验收部署

部署验证时间：2026-09-18 16:39（Asia/Shanghai）。用户已授权“部署到中国服务器上并隧道给我访问”。

## 访问

- 本机访问：<http://127.0.0.1:18081/keys>，使用原中国验收站的账号密码登录。
- SSH 隧道已在本机后台运行，仅监听 `127.0.0.1:18081`。
- 原验收站及其 `18080` 隧道保持运行。
- 新实例的数据库、Redis、应用数据目录和 JWT 签名密钥独立；改动只影响新实例。
- 网络仍为 `internal=true`，没有公开端口。可测试页面、权限和配置；真实上游模型调用、OAuth、外部邮件等受网络隔离限制。

## 版本和制品

原中国验收站为 `0.2.5-custom.1`，当前工作区基线为 `0.2.0`。为避免降级原站，本次建立独立候选实例；尚未把本功能合入 `0.2.5`，也没有发布正式 tag/Release。

| 项目 | 值 |
| --- | --- |
| 分支 | `custom/0.2.0` |
| 基线 HEAD | `53b12dc807efde190fcca71cd0501d259a78b652` |
| 候选版本 | `0.2.0-custom.3-candidate.multigroup.20260918t082757z` |
| 固定镜像 | `sub2api-custom:0.2.0-custom.3-candidate.multigroup.20260918t082757z` |
| 镜像 ID | `sha256:d48d7dc2904aac28e615a4053ceae5023fb09a80234b67fd5855f48b6adb8576` |
| 工作区文件清单 SHA-256 | `88ff579a462a50f5fe2e0806ec4f312ba9add12144e5d04400a1370cac1edbc5` |
| 源码归档 SHA-256 | `e15b2c7f8e7b070c3605b7787a1a942599c55d950ede10860b733b337fd01738` |
| 运行二进制 SHA-256 | `c49012073cff99ba8e1cf983df1c0aee2ed80c83fe8bbae1cca0e33e07189192` |

源码快照包含当前 tracked/untracked 非忽略文件，包括已有月卡、拼团和支付改动；不是仅归档 HEAD。未提交、推送或改动发布标签。后续新增的本部署记录不在构建快照内。

使用 Go 1.27.0、`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`、`-tags embed` 交叉编译，嵌入已通过完整检查的前端构建。运行基础镜像为服务器已有的 `sub2api-cn-runtime-base:20260918T022208Z`，构建前核对 ID 为 `sha256:87df7c0716c495cef1ce9ea369e3dd4f3fcb619e16f09bc46475ad116dd6da10`。实际 Dockerfile 保存在远端 `artifacts/final-Dockerfile`。

## 运行位置和数据副本

- 服务器：`yinan@game.game-mp.cn:1022`。
- 根目录：`/home/yinan/sub2api-staging/multigroup-20260918T082757Z`。
- Compose：根目录下 `runtime/compose.json`，包含私密环境配置，不打印或提交。
- Compose project：`sub2api-multigroup-cn`。
- 应用：`sub2api-multigroup-cn-sub2api-1`，ID `6bc4f7143c1ab605d2c52810f5aa243713c9c0747b7039c32bee5984b8c63768`。
- 数据库：独立容器 `sub2api-multigroup-cn-postgres-1`，DB `sub2api_multigroup`。
- 缓存：独立容器 `sub2api-multigroup-cn-redis-1`。
- 网络：`sub2api-multigroup-cn_isolated`，`internal=true`；当前应用 IP `172.26.0.4`。

原数据库约 5.6 GB，服务器部署前仅剩约 13 GB。使用一致性 `pg_dump`，排除四张表的数据：`ops_system_logs`、`ops_error_logs`、`prompt_audit_events`、`prompt_audit_jobs`；保留表结构以及用户、账户、密钥、订阅、用量和计费数据。恢复后约 604 MB，部署后剩余约 11 GB。

私密种子归档：`backup/staging-seed.dump`，大小 66,063,983 字节，SHA-256 `566282c3d31bc660784d1e64379ce93b565de437d9f79e87dc523910d01f1af4`。`pg_restore --list` 和空实例恢复成功，未改动原数据库。

副本额外配置：关闭自动备份、支付、定时账户测试、频道监测及提示词审计；保留原配置于种子归档。复制必要应用配置及模型价格文件，未复制运行日志。生成新数据库/Redis 密码及独立 JWT 密钥，不输出密钥值。

新版原站已移除 `groups.models_list_config`，旧基线仍需此字段。在副本中添加该 JSONB 字段并用 `codex_models_manifest_config` 初始化，操作保存在 `artifacts/clone-compatibility.sql`。比对本地 Ent 43 张表后无缺失字段。未更改原站 schema 或伪造迁移校验和；本操作仅用于旧基线独立验收。

迁移 `234_month_card_core.sql`、`235_month_card_billing.sql`、`236_api_key_multi_groups.sql`、`237_batch_image_billing_snapshot.sql` 在副本执行成功。旧密钥首组已回填到有序数组。

## 已验证与限制

- 部署前本地后端 unit/integration、lint/build 和前端完整检查均通过，见 verification.md；本候选工作区未运行 GitHub CI。
- 应用、PostgreSQL、Redis 均 healthy，自动重启计数为 0；应用主动重启约 5.1 秒恢复 healthy。
- 本机通过隧道访问首页、`/keys`、`/health`、公开版本接口和静态资源均返回 200。
- 用临时账号实际完成密码登录及登录后密钥管理请求，随后删除测试账号。原配置中的安装时管理员密码已失效（401），未重置任何已有账号密码。
- 用仅对候选实例有效的短期测试令牌验证：普通用户有序多组选组、管理员代配置、双方拒绝无资格组、首组兼容字段、调整顺序后共享总额度及窗口用量不变、缩减为单组后权限模式保留。
- 实际网关 `/v1/models` 返回 13 个去重模型；未向公网模型服务发送验收请求。
- 临时测试密钥已删除；测试账号/密钥保留正常软删除审计痕迹。
- 原有数据数量仍为：用户 151、账户 57、密钥 219、订阅 49（不计临时测试记录）。
- 原验收站应用/PostgreSQL/Redis 容器 ID 未变，均 healthy。日本生产未操作。
- 最后日志无缺列、迁移失败、fatal/panic；隔离网络导致的模型价格远程同步和订阅邮件失败为预期限制。
- 真实跨组推理、实际账单、长连接和上游异步任务仍需后续人工验收，不由页面/接口 smoke 检查替代。

远端证据：`artifacts/metadata.json`、`source-files.sha256`、`smoke-results.json`、`login-results.json`、`final-verification.json`，以及 `backup/archive.list`。

## 隧道重连和停止

控制 socket：`/private/tmp/sub2api-cn-multigroup-20260918T082757Z.sock`。Mac 重启或隧道退出后，可在本机重新运行以下命令；须先确保 18081 没有被其他进程占用。若应用容器重建，应先检查新的容器 IP 并替换转发目标。

```bash
ssh -F /dev/null -i ~/.ssh/gamemulti_codex_deploy -p 1022 \
  -o BatchMode=yes -o ConnectTimeout=15 -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -M -S /private/tmp/sub2api-cn-multigroup-20260918T082757Z.sock \
  -fNT -L 127.0.0.1:18081:172.26.0.4:8080 yinan@game.game-mp.cn
```

停止这条隧道：

```bash
ssh -F /dev/null -S /private/tmp/sub2api-cn-multigroup-20260918T082757Z.sock \
  -O exit -p 1022 yinan@game.game-mp.cn
```

如需停止候选，只对上述 `runtime/compose.json` 执行 `sudo docker compose -f <该文件> stop`。保留目录、数据库副本和种子归档，不恢复或覆盖原 staging 数据，也不清理其他项目镜像或容器。
