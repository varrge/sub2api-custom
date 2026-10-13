# 业务拒绝降噪：中国验收部署

2026-10-13 用户授权在中国测试服验收，已实现并部署。原周用量重置修正包含在本候选中。

- 入口：<http://127.0.0.1:18081/admin/ops>，使用原测试管理员账号。
- 源码：`fbde012273f5faebbe267dada2e8a7a6f46d6d74`，分支 `fix/ops-entitlement-rejections`，基于 `d8127439a`（周用量重置候选）。
- 版本：`0.2.15-custom.8-candidate.ops-rejections.fbde01227`。
- 固定镜像：`sub2api-custom:0.2.15-custom.8-candidate.ops-rejections.fbde01227`；镜像 ID：`sha256:bdefcb8ddbffc9c5ff1a166ceddd0c6359e54c85eee0c3d9351158a66b150506`。
- 二进制 SHA256：`c345f98f92b7985f7223bba849657a666d74479a68b092f925ce528e43e81c70`。
- 实例：`sub2api-multigroup-cn-sub2api-1`；仅替换应用。其他容器 ID 和启动时间不变，网络 internal=true，无公网端口发布，应用 healthy、零重启。
- 312 条数据库迁移及校验和完全不变，无新增迁移。

## 修改范围

具体规则见 [实现说明](ops-business-rejections.md)。明确的本地订阅不可用返回403并记入入口拒绝汇总；所有候选均不允许模型时返回403 `MODEL_NOT_ALLOWED`。真实容量不足保留503。前面分组拒绝而后面可用时正常选择，只有最终拒绝才标记汇总。真实上游403继续记录。

现有入口拒绝管理接口支持按 `subscription_unavailable` / `model_not_allowed` 筛选，保存用户、Key、原因、次数和首次/末次时间。不新增前端页面；运维错误明细不再逐条展示这些新请求。历史数据没有批量清理。

## 验收结果

本地先运行回归测试复现原错误，再验证修正：

- middleware、routes、handler、service、repository 五个包的完整 unit 测试通过；admin 入口拒绝筛选定向测试通过。
- 新增鉴权、汇总、续费、多分组拒绝测试的 race 检查通过。
- 独立代码审查没有剩余阻断项。
- 复用与上一候选完全相同的已验证前端产物，重新构建 linux/amd64 嵌入式二进制。

中国实例通过13项基础 HTTP 检查，以及10项针对本次修正的实际 HTTP 检查。创建仅供本次验收的临时用户、4个专属分组和4个Key，所有分组均无上游账号：

| 场景 | 实际结果 |
| --- | --- |
| 过期订阅重复 GET /models 三次 | 三次403，汇总计数3，零错误明细 |
| 多分组全部拒绝模型，重复两次 | 两次403 MODEL_NOT_ALLOWED，汇总计数2 |
| 一个分组订阅失效、另一个拒绝模型 | 403 SUBSCRIPTION_NOT_FOUND，汇总计数1 |
| 一个分组拒绝模型、另一个确实没有账号 | 503，保留一条非业务限制的故障明细 |
| 续费临时订阅，原Key再次 GET /models | 200，Key和绑定不变 |
| 管理接口按两种拒绝原因筛选 | 200，分别读到4次和2次 |

6次正常拒绝都进入汇总；唯一故障明细为容量503，未选择上游账号。没有上游请求、用量记录或余额扣费。真实供应商403保留由本地回归验证，中国隔离网络未做真实供应商调用。未执行浏览器交互验收；穿透路由与 JS/CSS 摘要通过 HTTP 核对。

验收结束清理仅属于本次临时用户的订阅、Key、分组和测试日志。清理前后业务记录总数一致：用户154、分组36、Key227、旧订阅49、账号57、月卡0。未修改原客户订阅或额度。临时密钥仅驻留进程内存，凭据没有落盘；删除记录触发原有缓存失效机制。

## 备份与回滚

- 远端：`/home/yinan/sub2api-staging/ops-rejections-20261013T072823Z`。
- 本机备份与证据：`/Users/varrge/Downloads/Sub2API-中国业务拒绝降噪验收-20261013T072823Z`，目录权限0700，包含私密配置。
- 数据库 dump 64,156,710 字节；`pg_restore --list` 与本机10个备份文件SHA256校验通过。
- 回滚镜像：`sub2api-restore:ops-rejections-20261013t072823z`，恢复上一版周用量重置候选。
- 将远端 `backup/compose.rollback.json` 恢复至 `/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json`，在原Compose目录以项目 `sub2api-multigroup-cn` 执行 `up -d --no-deps --force-recreate --pull never sub2api`。无需恢复数据库。

复用穿透：`127.0.0.1:18081 → 172.26.0.4:8080`，控制 socket `/private/tmp/sub2api-cn-weekly-reset-20261013T065123Z.sock`。Mac重启后需重开；容器重建后核对IP。

本次没有部署美国服务器、合并发布分支或打tag。
