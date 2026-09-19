# 密钥模型限制：中国验收部署

2026-09-19（Asia/Shanghai），按用户「中国服务器创建隧道给我测试」授权，升级现有多分组隔离验收实例。

- 访问：<http://127.0.0.1:18081/keys>，原中国验收账号密码不变；编辑密钥最底部开启模型限制。
- 版本：`0.2.6-custom.2-candidate.model-limits.87fdc8242`。
- 源码：`feature/api-key-model-limits`，`87fdc824250996e488c52adf8d9f73085d35d569`，通过 `git archive` 取得干净提交；包括前一提交的路由错误模型日志和 Codex 被动工具预检修复。
- 镜像：`sub2api-custom:0.2.6-custom.2-candidate.model-limits.87fdc8242`。
- 镜像 ID：`sha256:6a8da4698ed822e0e65c346605730b6b5e41d9ac6de222aec35305a86c9d4264`。
- 二进制 SHA-256：`b468a4681df975dd3644e62e8f92734eb5f2e01171f9e42babcd21b4587cb398`。
- 源码归档 SHA-256：`f499def1bccea664b7aca66259d9846aab14eb8f58d5e26dcd48f5884fee1e90`。

## 构建与部署

Node 20.20.2 / pnpm 9 前端构建通过；Go 1.27.0 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 -tags embed` 构建，并显式注入候选版本和源码 commit。使用服务器已有且核对过摘要的 `sub2api-cn-runtime-base:20260918T022208Z` 组装应用镜像。

只对 Compose project `sub2api-multigroup-cn` 的应用执行 `up -d --no-deps --force-recreate sub2api`。活动配置仍是 `/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json`，含私密配置，不输出或提交。

- 应用容器：`7bea7897badd340e4d458bc809af617919098c66e6889de69759cd7b97f7b4c8`。
- 应用 IP：`172.26.0.4`，无公开端口。
- PostgreSQL、Redis、原 staging 及中国其他项目容器 ID 均未变化。
- 网络 `sub2api-multigroup-cn_isolated` 保持 `internal=true`。
- 迁移 244 成功；历史密钥模型限制默认关闭。
- 初次替换约 7.8 秒健康，主动重启后约 6.8 秒健康；运行版本、镜像和二进制摘要一致。

## 备份与验证

远端记录根目录：`/home/yinan/sub2api-staging/key-model-limits-20260919T145315Z`。

替换前保存私密 Compose、一致性 PostgreSQL 归档（65,916,314 字节，`pg_restore --list` 成功）及精确应用回滚镜像 `sub2api-restore:key-model-limits-20260919t145315z`。备份不覆盖现有文件，不恢复数据库。

部署前总记录数：users 154、accounts 57、api_keys 222、user_subscriptions 49。迁移后相同；功能验收创建并软删除一把临时密钥，保留审计痕迹。重启后非删除记录数与部署前相同：users 151、accounts 19、api_keys 158、user_subscriptions 24。

实际 HTTP 验证通过：

- 未登录不能读取模型选项；有资格的组 3、4 合并得到 12 个模型；用户与管理员目录一致。
- 临时密钥保存与回读限制、普通编辑保留限制、开启但空列表拒绝、关闭限制。
- 未勾选模型在转发前被拒绝；网关目录仅保留勾选模型。
- 管理员更改分组顺序与模型限制不会改变整把密钥额度和窗口用量；鉴权缓存及时刷新。
- 本机隧道首页、keys 页面、health、版本接口和模型限制静态资源内容校验通过；重启后仍可访问。

本次仅中国隔离候选验收；未推送或发布 tag/Release，尚未运行该提交的 GitHub CI，未修改日本生产。隔离网络不能进行真实上游消耗、OAuth 或邮件验收。

远端 `artifacts/` 保存源码、镜像上下文、摘要、preflight、deployment、model-limits-smoke 及 final-verification；备份在 `backup/`。

## 隧道

本机后台 SSH 仅监听 `127.0.0.1:18081`，控制 socket：`/private/tmp/sub2api-cn-key-model-limits-20260919T145315Z.sock`。

重启 Mac 后如需重建，先确认端口未占用：

```bash
ssh -F /dev/null -i ~/.ssh/gamemulti_codex_deploy -p 1022 \
  -o BatchMode=yes -o ConnectTimeout=15 -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -M -S /private/tmp/sub2api-cn-key-model-limits-20260919T145315Z.sock \
  -fNT -L 127.0.0.1:18081:172.26.0.4:8080 yinan@game.game-mp.cn
```

容器重建后先核对应用 IP，再更新转发目标。
