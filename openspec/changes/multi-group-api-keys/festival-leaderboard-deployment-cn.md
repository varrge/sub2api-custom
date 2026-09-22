# 双节消费榜：中国测试部署

日期：2026-09-22。用户授权“部署到中国服务器上”。

- 访问：<http://127.0.0.1:18081/keys>，原 SSH 隧道继续使用。
- 部署分支：`feat/cn-festival-leaderboard`。未合并分支、未推送、未创建 tag。
- 保留服务器当日黑金主题基线 `727a8d367`，叠加已审查的消费榜功能。
- 源码：`49263a9f0b2a61799a313e870d0eaaf3d3fc28c4`。
- 版本：`0.2.7-custom.7-candidate.festival.49263a9f0`。
- 镜像：`sub2api-custom:0.2.7-custom.7-candidate.festival.49263a9f0`。
- 镜像 ID：`sha256:fbaacfa6d3c96909e1dafa93f732489401b55ae5ec7808b4744ec40cf7bd7d3b`。
- 二进制 SHA-256：`a001df1f085cf864578f694a4ef4b8adc0f2f8f825151c925f16b81bba0e0949`。
- 源码归档 SHA-256：`4657abe31ffaa721bde82aa220a6b78b8b2e67ace5b5ee2579b3887ac19a8283`。

## 部署与回滚

- 目标：`yinan@game.game-mp.cn:1022`，Compose project `sub2api-multigroup-cn`。
- 备份与证据：`/home/yinan/sub2api-staging/festival-20260922T100719Z`。
- 备份包括完整 PostgreSQL dump、私密 Compose、应用配置；回滚镜像 `sub2api-restore:festival-20260922t100719z` 已验证二进制与切换前一致。
- 数据库备份：65365431 字节，SHA-256 `7435d6ee7759b0ba6d74ec0623024dd43cd65b18f2bf0bf98c28b68c57a5537a`，归档目录可读取。
- 没有数据库迁移；只替换应用镜像。失败时自动恢复原应用镜像，不恢复或覆盖数据库。
- 约 7.8 秒后 healthy，重启次数 0；其他容器 ID 不变。
- 网络仍 `internal=true`，不公开宿主机端口。日本生产未操作。
- 用户/账号/API 密钥/订阅切换前后计数一致。

## 验证

- 当前主题基线下前端 335 个文件 / 2498 项测试通过，ESLint 和生产构建通过。
- 后端榜单、插件能力及服务装配检查通过。之前榜单独立 PostgreSQL 和 race 验证通过，候选后端逻辑相同。
- 真实接口：未登录返回 401；普通用户及管理员可读；活动时间固定，当前返回 upcoming 与空榜，无私人标识泄漏。
- 原有密钥、订阅、管理账号、用量和模型广场只读 smoke 检查通过。
- 当前活动未开始：2026-09-25 00:00 至 2026-10-08 00:00，北京时间。页面显示即将开始，无虚构榜单。
- 详情见 [消费榜说明](../../../docs/festival-leaderboard.md)。

浏览器实测桌面明暗主题、375px 和 320px：榜单接口 200、入口顺序正确、弹窗不溢出、Esc 关闭及焦点恢复正常、无页面 JS 异常。320px 时原有密钥列表的一处内容容器超宽约 8px，位于榜单之外；没有将此问题算作全页面适配通过。

## 下拉浮层更新（2026-09-22）

- 用户要求：根据截图将居中大弹窗改成右上角下拉浮层，并让 Kimi 实现。
- Kimi 完成组件、交互和文案；主代理补全验证。仅改前端，消费统计和 API 未改。
- 应用提交：`b96466f0bf21c6d11e010129c144654c12647f99`；版本：`0.2.7-custom.7-candidate.festival-popover.b96466f0b`。
- 镜像：`sub2api-custom:0.2.7-custom.7-candidate.festival-popover.b96466f0b`；ID：`sha256:f583f6e7a4efcaf69905f8403297a7c7223862a9a9c2645cefefd68c1de26532`。
- 二进制 SHA-256：`88af058ab517848b7a6a50dfaf4fda868403884f6954dd1b26ce0ba838e07578`。
- 备份/证据目录：`/home/yinan/sub2api-staging/festival-popover-20260922T102604Z`。
- 回滚镜像：`sub2api-restore:festival-popover-20260922t102604z`，数据库 dump `65349914` 字节，归档可读。
- 只重建应用，7.9 秒恢复 healthy，其他容器及记录计数不变。
- 27 项相关前端测试、ESLint、类型编译和构建通过；本地完整活动榜与中国服务器 upcoming 实测均通过。
- 浏览器确认 400px 宽（手机自适应）、最大高 560px，无遮罩、不锁页面滚动；按钮切换、外部点击、Esc 及缩放定位正常。
- 原隧道 <http://127.0.0.1:18081/keys> 继续使用；未合并分支、未推送、未打 tag。
