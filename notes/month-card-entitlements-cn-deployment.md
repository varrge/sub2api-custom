# 月卡权益管理：中国测试部署

2026-10-10，按用户要求部署中国测试服务器并开启本机 SSH 隧道。

- 入口：http://127.0.0.1:18081/admin/group-buy ，登录原测试管理员账号后选择“月卡权益”。
- 隧道控制 socket：`/private/tmp/sub2api-entitlements-cn.sock`，仅监听本机 127.0.0.1:18081，目标 172.26.0.4:8080。
- 中国实例：`sub2api-multigroup-cn`，应用 `sub2api-multigroup-cn-sub2api-1`。
- 部署源码：`/Users/varrge/workspace/kwRedeem/sub2api-entitlements-cn`，分支 `feat/cn-monthcard-entitlements-20261010`。
- 基线：`37a0488a4a157e2138878aaeda5ec505a49173eb`。当前旧开发目录基于 0.2.0，因此将权益功能单独移植到 0.2.14；保留规则管理、冻结期、排行榜和现有支付功能。
- 版本：`0.2.14-custom.2-candidate.entitlements.0942ce655126`。
- 固定镜像：`sub2api-custom:0.2.14-custom.2-candidate.entitlements.0942ce655126`，镜像 ID `sha256:8807453014c3ffe90ed4bb6e8c16549f7f5c2d2e7d9305fa34126cf08f5160bb`。
- 二进制 SHA256：`8a5aff193f2b6d3a8e7b0c7f6922dc1a21ae1876a62867c8d00d0e3975874e6f`。
- 源码清单 SHA256：`0942ce65512670576292b587d27d3d12202b2aece41135e67d1d211d85106375`。源码快照包含本次未提交功能；未创建 commit、tag 或正式 Release。

## 功能与兼容

管理员默认浏览有有效月卡的拼团，支持分组、状态、客户名称/邮箱/ID 搜索和分页；团详情列出客户、订单及独立额度用量。独购卡有独立页签。客户信息接口限定管理员，普通用户 403，匿名 401。

冻结中的未失效月卡仍在权益总览中，当前可用额度为 0。冻结与解冻后的到期时间、周用量均沿用现有计算，冻结期结束按既有规则自动解冻。

## 验证与数据范围

- 前端 113 项相关测试、类型检查、定向 ESLint 和 Vite 生产构建通过。
- 完整 monthcard 包使用一次性 PostgreSQL 测试通过，包括冻结超过原到期日、按冻结期边界解冻、退款、分页、历史团和用量合计；handler 与路由权限测试通过。
- 中国应用 healthy，重启次数 0，运行二进制哈希匹配。
- 19 项实际 HTTP 检查通过；规则、冻结期、优惠码、排行榜接口均正常。
- 实际页面在桌面 1440px 和手机 390px 验证通过，无横向溢出或页面脚本错误，拼团/独购切换正常。
- 核心数据计数不变：用户 154、账号 57、Key 227、订阅 49、月卡商品 2。
- **测试库无拼团、购买月卡或额度扣费记录，因此实际页面显示空列表；服务器上的有数据详情验收未执行。成员与用量逻辑已在本地真实 PostgreSQL 和组件测试验证。**
- 无新增迁移，现有迁移校验和全部匹配；保留原 4 条历史记录。其他容器的 ID 和启动时间不变，网络保持 internal，无新增公网监听。

## 备份与回滚

- 远端制品和备份：`/home/yinan/sub2api-staging/entitlements-20261010T025806Z`。
- 本机持久证据和完整备份：`/Users/varrge/Downloads/Sub2API-中国月卡权益部署-20261010`（含私密配置，目录权限 0700）。
- PostgreSQL 一致性自定义格式备份 64,869,046 字节，pg_restore --list 成功；本地副本全部 SHA256 校验匹配。
- 旧应用回滚镜像：`sub2api-restore:entitlements-20261010t025806z`。
- 回滚配置：远端 `backup/compose.rollback.json`。恢复至 `/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json` 后，以项目 `sub2api-multigroup-cn` 执行 `up -d --no-deps --force-recreate --pull never sub2api`。保持原 Compose 目录解析相对挂载。
- 应用回滚无需恢复数据库。此次未修改账户密码或商品价格，未创建测试订单。

## 隧道重连

确认 18081 未被占用后执行：

```sh
ssh -F /dev/null -i ~/.ssh/gamemulti_codex_deploy -p 1022 \
  -o BatchMode=yes -o ConnectTimeout=15 -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -M -S /private/tmp/sub2api-entitlements-cn.sock -fNT \
  -L 127.0.0.1:18081:172.26.0.4:8080 yinan@game.game-mp.cn
```

应用容器重建后应先核实目标 IP。隧道随本机 SSH 进程运行，Mac 重启后需重新开启。

## 正式版本合并

用户授权合并与打标签后，同步远端确认最新正式基线为 `v0.2.15-custom.1`，因此本功能合入 `custom/0.2.15`，版本更新为 `v0.2.15-custom.2`。月卡模块在两个基线之间没有变更；新增前端接口和整组月卡测试进入 CI 必跑列表。

中国测试站仍运行上文已验收的 0.2.14 候选。新标签保留 0.2.15 的其他更新；推送标签触发正式构建发布，不自动更新服务器。
