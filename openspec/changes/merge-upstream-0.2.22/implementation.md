# 官方 v0.2.22 自定义适配

日期：2026-11-17（Asia/Shanghai）。用户授权开始适配，未授权本轮直接发布 tag 或更新美国生产。

后续复验更新：同日的[充值与退款复验](revalidation.md)发现四项既有支付问题，现已完成[修正与专项验收](payment-fixes.md)。中国验收仍是本文件记录的原候选镜像，尚未包含本次支付修正；当前发版判断以修正记录为准。

## 固定基线

- 自定义：`4e4f957af39e43d85263a9504553b845bea9ad40`，本地已合入官方 0.2.21。
- 官方：`v0.2.22` → `b0fa9f3e6eeab1968a53e1289b69daa4bb40ca2b`。
- 工作分支：`codex/merge-upstream-0.2.22`。
- 工作树：`/Users/varrge/.codex/worktrees/image-moderation-error/sub2api-custom`。

## 适配决定

1. 保留 ForcedServiceTier 单选设置及旧 Fast API 兼容，保留免费 FAST/UF 与自定义倍率；保留已有 GPT-5.6 Sol UF 2 倍、Astra / GPT-6.1 Sol UF 6 倍。官方两布尔开关不替代现有用户配置。
2. 官方批量扣费仅处理普通命令；含月卡、共享额度、固定预留、已接受异步图片的批次在同一用户通道内按顺序逐条结算。原始批量仓储也在 SQL/去重之前拒绝这些命令，避免其他调用者绕过保护。
3. 新扣费包装器显式委托图片权益快照与 Grok 持久视频快照；后台月卡恢复通过原始仓储启动，保留恢复回调。
4. 保留严格分币归一化的支付宝退款幂等与恢复查询。
5. 合入调度 bucket 加载优化、冷却账号快照、分组 RPM 作用域及同步失败保护；保留利润门槛、分组兜底、多组密钥、企业价格与额度。
6. 合入 Gemini 协议、Vertex 默认价区域倍率、工具流与显式定价覆盖修复。
7. 保留官方新增 SQL 完整文件名以维护校验和；现有自定义单选字段继续为应用配置源。部署前验证迁移。

## 完成状态

40 个原始文本冲突全部解决，Ent / Wire 已重新生成，最终暂存内容无冲突与空白错误。独立复核未发现确认的阻塞问题（Critical / Important / Minor 均为 0）。

官方模型测试按现有自定义合同适配：不支持强制 UF 的模型默认不增加 UF 倍率；支持与倍率分别断言，避免误把 Sol 5.6 的 2x 判为不支持。没有为使测试通过而改变生产计费规则。

## 检查结果

- `go test -tags=unit ./... -count=1 -timeout=15m`：通过。
- `CI=true go test -tags=integration ./... -count=1 -timeout=15m`：通过；真实 PostgreSQL 18.1 / Redis 8.4 容器运行，完整迁移成功；为 scheduled / activity / enterprise / model-access 配置了独立测试 DSN。
- 月卡、推送、投票、教程、支付回调五个 package 的 `go test -race`：250 项（含子测试）通过、0 跳过；为各自 PostgreSQL 测试配置了 DSN。
- 新增 lane 真实数据库回归：普通 300 请求并发、月卡并发与重放、共享额度耗尽与重放均通过。共享额度例：12 请求、原价共 12、客户成本共 6，共享池原价消耗 2.5、抵扣 1.25，个人余额 10 → 5.25，Key 用量 6；重复提交不改变账务。
- 原始批量仓储在 SQL / 去重之前拒绝自定义结算命令；单元测试验证月卡、共享额度、固定预留、接受后的异步图片及媒体权益快照委托。
- `golangci-lint v2.14.0 run --timeout=30m`：0 issues。
- 前端全量 ESLint、typecheck、生产构建：通过。
- `make test-frontend-critical`：72 文件 / 1092 项通过；另有合并相关与自定义业务回归 49 文件 / 765 项通过（与关键集有重叠，不相加）。
- 部署 shell 检查、Apple lifecycle fixture、Compose 安全 / Gateway 环境 / 资源 / simple-mode、Caddy 缓存策略、Release 工作流规则均通过；Release Python 10 项测试通过。
- Linux amd64、Go 1.27.4、嵌入前端的 release 候选二进制构建通过。

全量 Go 命令中仍有仓库预设的可选测试跳过（包括需独立 campaign / security-audit DSN 或真实外部供应商的项目）；上列五个专门数据库 race suite 已全部实际执行。没有将跳过的测试计作通过，也未发起真实上游付费请求或执行生产压测。

临时日志与分工报告：`/tmp/sub2api-022-*.log`、`/tmp/sub2api-022-*.jsonl`、`/tmp/sub2api-022-*-report.md`、`/tmp/sub2api-022-review.md`。

## 中国隔离验收

- 原验收镜像：`sub2api-custom:0.2.21-custom.1-candidate.20261113-2293e1906`。镜像标记的源码树与本轮起点 HEAD 完全一致。
- 候选版本：`0.2.22-custom.1-candidate.20261117`。
- 候选源码树：`3104ea3f32fe0981947cc7f6a368deff0e8976a2`；最终提交相对此树只补充本文验收记录。
- 候选镜像：`sub2api-custom:0.2.22-custom.1-candidate.20261117-3104ea3f`。
- Linux 二进制 SHA-256：`a78569eb1a3666c547d33df75b60aa73df94e475b7599827449ea9feaefc3c13`。
- 服务器目录：`/home/yinan/sub2api-staging/upgrade-0222-20261117-3104ea3f`。
- 应用：`sub2api-multigroup-cn-sub2api-1`，仅重建这一应用。
- 网络：`sub2api-multigroup-cn_isolated` 仍为 internal；未开放上游公网出口。
- 备份：上述目录的 `backup/`，含有效 PostgreSQL dump / archive list、应用数据及原 Compose 配置。
- 精确回滚镜像：`sub2api-restore:pre-0222-20261117-3104ea3f`，对应 override 位于 `backup/rollback.override.json`。
- 数据前后均为：users 156、accounts 58、api_keys 227、user_subscriptions 49。
- 其他 30 个运行容器 ID 与启动时间不变。

验收实际检查：health、登录页及其静态资源、后台页面；临时管理员密码登录与个人信息；账号、分组、渠道、设置、用量、教程和投票 API；12h 缓存价格与 UF 自定义倍率的正值 / 零 / null 回读；四种强制档位、免费 Fast / UF、旧 Fast 别名及显式单选优先级；两份官方 251 迁移及既有免费标签迁移；应用重启与管理员会话保持。全部通过，最终 Docker healthy / RestartCount 0，日志没有 fatal / panic / 迁移失败。

旧验收脚本引用的测试邮箱已不在当前数据库，容器 bootstrap 凭据也无法登录；本轮未修改任何现有管理员。使用独立临时管理员与隔离测试用的后台访问状态完成验证，随后删除该用户及测试状态，核心计数恢复一致。测试分组和渠道亦已删除。未进行浏览器人工交互验收。

服务器切换前可用空间约 4.6 GiB，因此在本机交叉编译并复用已验证运行时构建候选镜像；未清理任何旧备份或其他项目镜像。

## 发布边界与性能范围

本轮完成本地适配与中国验收；未 push、未创建 tag、未触发 GitHub Release，未改动美国生产。

带分组且有 RawCost 的普通收费请求同样经过共享额度判断，因此仍按用户顺序逐条结算。它们暂未获得官方批量 SQL 的吞吐收益；保留这一边界，直到批量实现能覆盖同等自定义账务效果。其他上游调度、协议与稳定性改进照常合入。
