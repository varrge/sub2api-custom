# 紧凑拼团大厅：中国测试部署

2026-10-12，按用户要求部署中国测试实例，并打开本机 SSH 穿透。

- 大厅入口：<http://127.0.0.1:18081/group-buy>，使用原测试站账号登录。
- 部署源码：`deeb96b09bef1c0937c3e2e4b643063304a13b3f`，分支 `feat/compact-group-buy-hall`。
- 版本：`0.2.15-custom.6-candidate.hall-compact.deeb96b09`。
- 页面采用紧凑布局和项目原有米色／铜金主题，移除大厅卡片的额度阶梯展开区，保留详情介绍。
- 实例：`sub2api-multigroup-cn-sub2api-1`；只重建应用，其他容器、数据库迁移和核心记录数不变。
- 镜像：`sub2api-custom:0.2.15-custom.6-candidate.hall-compact.deeb96b09`。
- 运行二进制 SHA256：`020df5c7459ffb316c70259e5515f123fcc15c2b1815063e497a2d56e996e67d`。

前端 92 项相关测试、类型检查和生产构建通过。运行容器 healthy、零重启，版本和二进制摘要匹配。实际接口鉴权与公告读取正常；通过隧道获取的 9 个 JS/CSS 文件与本地构建摘要一致。

实际浏览器在 1440px 和 390px 验证无横向溢出、无脚本错误，未模拟 API。**测试库目前有 2 个商品、0 个拼团，大厅显示空状态；此次未新增演示订单或拼团。** 有数据卡片布局已在本地模拟数据验证，不能将空大厅验收视为服务器有数据卡片验收。

## 备份与回滚

- 远端目录：`/home/yinan/sub2api-staging/hall-compact-20261012T085535Z`。
- 本机备份和证据：`/Users/varrge/Downloads/Sub2API-中国紧凑大厅部署-20261012`，目录权限 0700，备份包含私密配置。
- PostgreSQL dump 为 64,357,130 字节，目录校验和下载后的文件 SHA256 校验均通过。
- 回滚镜像：`sub2api-restore:hall-compact-20261012t085535z`。
- 将远端 `backup/compose.rollback.json` 恢复到 `/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json`，使用原目录和项目 `sub2api-multigroup-cn` 执行 `up -d --no-deps --force-recreate --pull never sub2api`。无需恢复数据库。

## 穿透

监听 `127.0.0.1:18081`，转发至中国应用 `172.26.0.4:8080`。控制 socket：`/private/tmp/sub2api-cn-hall-compact-20261012.sock`。隧道依赖本机 SSH 进程，Mac 重启后需重开；容器重建后需核对目标 IP。

```sh
ssh -F /dev/null -i ~/.ssh/gamemulti_codex_deploy -p 1022 \
  -o BatchMode=yes -o ConnectTimeout=15 -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -M -S /private/tmp/sub2api-cn-hall-compact-20261012.sock -fNT \
  -L 127.0.0.1:18081:172.26.0.4:8080 yinan@game.game-mp.cn
```

本次没有合并发布分支或打 tag。临时验证凭据文件已删除。
