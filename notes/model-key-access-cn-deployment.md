# 按模型管理 Key：中国测试实例部署

2026-09-23，按用户「部署到中国服务器」指令，更新现有中国隔离测试实例。

## 访问与版本

- 入口：<http://127.0.0.1:18081/keys/model-access>，也可从 API 密钥工具栏点击「按模型管理」。沿用现有账号。
- 版本：`0.2.7-custom.10-candidate.model-access.592719912eab`。
- 工作树：`sub2api-model-access`，分支 `feat/model-key-access`；基线 `a3e21abe8fb4ec1b25d10833572bbba1701f4700`，包含尚未提交的功能改动，以源码归档及逐文件哈希锁定内容。
- 镜像：`sub2api-custom:0.2.7-custom.10-candidate.model-access.592719912eab`。
- 镜像 ID：`sha256:828b78d84c0a320fbf88b71f4091025bfa46da706a538a447a76328586dbb33e`。
- 二进制 SHA256：`7b486f71bc160b78a4dcc9d1a0816d9485590a64594710ac11252af4d98d7406`。
- 源码归档 SHA256：`08bb361e5a4fdd289199f8c165edc3a86a1ebd6aa5047870e542d1e6eb9caf7e`。

Go 1.27.0、`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 -tags embed` 编译并嵌入本次生产构建的前端；使用已校验摘要的中国运行时基础镜像。没有新增数据库迁移。

## 部署范围与备份

- 服务器：`yinan@game.game-mp.cn:1022`。
- 活动 Compose：`/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json`，含私密配置，不输出或提交。
- 仅替换 project `sub2api-multigroup-cn` 的应用 `sub2api-multigroup-cn-sub2api-1`；其他容器 ID 不变。
- 应用 IP `172.26.0.4`，网络 `internal=true`，无公网端口映射。原 SSH 隧道 `18081` 继续可用，ControlPath `/private/tmp/sub2api-catalog-cn.sock`。
- 部署记录与备份：`/home/yinan/sub2api-staging/model-access-20260923T102744Z`。
- 一致性 PostgreSQL dump：65,379,823 字节，`pg_restore --list` 验证通过；另保存应用数据、原 Compose、私密 inspect 和精确旧镜像。
- 回滚镜像：`sub2api-restore:model-access-20260923t102744z`。回滚仅切换应用镜像，保留数据库，不覆盖恢复数据库。

首次切换的验收脚本误将空名单必须返回 `models: []` 作为条件，实际 JSON 会省略空字段，导致脚本报错并自动恢复旧镜像。临时 Key 已清理；确认旧版健康后，将验收条件修正为缺省字段等价空数组，重新部署同一制品并通过全部检查。两次验收共创建并软删除两把临时 Key，保留正常审计痕迹。

## 验证结果

- 前端 72 项相关测试、修改文件 ESLint、类型检查、生产构建，以及后端定向测试和本地真实 PostgreSQL 事务测试通过。
- 应用约 7.8 秒进入 healthy；最终重启次数为 0，运行版本及二进制摘要匹配。
- 真实 HTTP：未登录拒绝、跨用户修改拒绝、响应无密钥凭据、关闭限制转 deny、单 Key 与按模型页面双向回读、保留其他模型、旧批量修订值 409、旧单 Key 修订值 409、空 allow 禁止全部以及重新允许模型均通过。
- 实际网关目录从 17 个模型，经禁止全部变为 0，再允许所选模型后变为 1，验证鉴权缓存刷新；未发起外部推理消费。
- 当前用户、Key、订阅、用量、商品及原购买规则接口通过。
- 本机隧道连接真实服务器，在 1440、390、320 像素浏览器中检查工具栏入口与新页面：无横向溢出或运行时错误。浏览器验收阻止写请求；真实写入验证仅对临时 Key 执行。
- 原有未删除记录数量保持：users 151、accounts 19、api_keys 158、user_subscriptions 24。总 Key 数因两条软删除测试记录由 225 变为 227。
- 为恢复磁盘余量，逐文件与保留的 `artifacts/image.tar.gz` 核对后，删除六个重复解压构建目录；源码包、镜像、备份均保留。最终可用约 1.06 GiB，清单在 `artifacts/build-context-cleanup.json`。

本地证据：`/tmp/sub2api-model-access-deploy-20260923T102744Z`；远端 `artifacts/` 包含部署、接口验收、最终健康和清理记录。部署步骤未推送、发布 tag、合并或部署日本生产。

## 正式标签

随后按用户「打 tag」指令准备 `v0.2.7-custom.11`。发布源码的业务逻辑与中国候选制品一致；仅补充部署记录、将新页面及 PostgreSQL 事务测试纳入发布 CI、更新接口契约测试中的策略修订字段，并按静态检查规范显式忽略测试数据库关闭错误。中国实例继续运行上述候选镜像，标签发布不再次替换实例。

发布前全量前端测试 2613 项、前端 CI 的 282 项关键测试、Go 静态检查、数据库集成测试、支付并发回归及发布脚本检查通过。本地发布检查证据保存在 `/tmp/sub2api-custom11-release`；标签触发仓库既有 Release 工作流，发布产物须通过标签所指提交的完整 CI。
