# 周用量重置同步扣减累计用量：中国测试部署

2026-10-13，按用户要求部署中国测试实例，并开启本机 SSH 穿透。

- 入口：<http://127.0.0.1:18081/admin/group-buy>，使用原测试管理员账号登录后选择「月卡权益」。
- 部署源码：`08d452f2fcf150cf2d0d91e305dd5779c5a43a24`，分支 `fix/monthcard-weekly-reset-total`，基于 `v0.2.15-custom.8`。
- 版本及固定镜像：`sub2api-custom:0.2.15-custom.8-candidate.weekly-reset.08d452f2f`。
- 镜像 ID：`sha256:9c1a66e17764d402cdb584e29ab377ba997c5587ff4498c42e82c8d8e48830a3`。
- 运行二进制 SHA256：`a529ffc1c856bcd4598242645acaebff8dc91a54a43bafc8bca0084e6b58e6ef`。
- 实例：`sub2api-multigroup-cn-sub2api-1`；仅重建应用，保留 PostgreSQL、Redis 与其他容器。

手动清零本周已用时，累计已用同时扣除本次清除的周用量，最低为 0。总额度 400、累计已用 55、本周已用 30，重置后为本周 0、累计 25、总剩余 375。单张与批量行为一致，弹窗显示预计结果。自动换周仍保留累计用量，保存前尚未结算的请求按原周代次结算并继续增加累计。

## 验证与数据范围

部署前真实 PostgreSQL 的定向测试及 race 检查、25 项前端相关测试、类型检查和 ESLint 已通过；此次前端生产构建与嵌入前端的 linux/amd64 编译通过。

运行容器 healthy、零重启，版本和二进制摘要一致。13 项 HTTP 检查通过，包括月卡权益查询，以及重置接口的匿名 401、普通用户 403、管理员空选择 400。通过本机隧道获取的 7 个 JS/CSS 文件与本地构建摘要一致，包含新的重置说明。

核心记录数保持不变：用户 154、账号 57、Key 227、旧订阅 49、月卡商品 2、拼团 0、月卡 0、扣费分配 0。没有对现有用户执行额度重置，没有新增演示订单或月卡。**测试库暂无已购月卡，因此无法在该实例验证有数据的重置弹窗；55→25 等行为已经本地真实 PostgreSQL 和组件测试验证。** Browser 当前无可连接浏览器，未执行浏览器交互验收。

与原先紧凑大厅候选相比，仅补入已合并的 `251_api_key_model_group_routes.sql`，为 API Key 增加默认空对象的模型分组选择字段。已有 311 条迁移及其校验和保持不变，包括 4 条历史迁移；部署后共 312 条。周用量修正本身没有新增迁移。其他容器的 ID 与启动时间均不变；应用网络保持 internal，无公网端口绑定。

启动日志有一条远程价格文件校验失败：隔离网络无法解析 `raw.githubusercontent.com`。保留原有网络隔离，不影响本次管理页面和额度重置验证。

## 备份与回滚

- 远端目录：`/home/yinan/sub2api-staging/weekly-reset-20261013T065123Z`。
- 本机证据及备份：`/Users/varrge/Downloads/Sub2API-中国周额度重置部署-20261013T065123Z`，目录权限 0700，包含私密配置。
- PostgreSQL dump 为 64,213,555 字节，`pg_restore --list` 验证成功；本机备份全部 10 个文件的 SHA256 校验通过。
- 回滚镜像：`sub2api-restore:weekly-reset-20261013t065123z`，对应原紧凑大厅候选 `0.2.15-custom.6-candidate.hall-compact.deeb96b09`。
- 将远端 `backup/compose.rollback.json` 恢复到 `/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json`，在该目录以项目 `sub2api-multigroup-cn` 执行 `up -d --no-deps --force-recreate --pull never sub2api`。原应用兼容新增字段，无需恢复数据库或删除该字段。

## 穿透

本机 `127.0.0.1:18081` 转发至中国应用 `172.26.0.4:8080`。控制 socket：`/private/tmp/sub2api-cn-weekly-reset-20261013T065123Z.sock`。隧道依赖本机 SSH 进程，Mac 重启后需重开；容器重建后需核对目标 IP。

```sh
ssh -F /dev/null -i ~/.ssh/gamemulti_codex_deploy -p 1022 \
  -o BatchMode=yes -o ConnectTimeout=15 -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -M -S /private/tmp/sub2api-cn-weekly-reset-20261013T065123Z.sock -fNT \
  -L 127.0.0.1:18081:172.26.0.4:8080 yinan@game.game-mp.cn
```

本次未合并发布分支或打 tag。接口验证的短期凭据仅驻留验证进程内存，未落盘。
