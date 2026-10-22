# 企业模式：中国独立验收部署

> 本页仅记录旧 0.2.0 企业候选部署；后续 `v0.2.17-custom.7` 源码移植不会自动更新此验收实例，见 [port-0217.md](port-0217.md)。

> 2026-10-22 00:43 更新：18082 已升级到 `0.2.0-custom.3-candidate.enterprise.effects.20261022`。“特权设置”支持 A/B/C/D 四种欢迎动效，企业与普通用户规则分别选择；旧配置保留 A。以下历史部署信息保留，最新制品见文末更新记录。

时间：2026-10-21 17:45（Asia/Shanghai）。用户已授权“部署到中国服务器开穿透给我测测”。

## 入口与范围

- 本机入口：http://127.0.0.1:18082/login ，使用原中国旧验收站账号密码。
- SSH 隧道在本机后台运行，只监听 127.0.0.1:18082；控制 socket 为 `/private/tmp/sub2api-cn-enterprise-DMQW2gg2.sock`。
- 独立企业验收实例；现有 18081 的 0.2.17-custom.6 和旧 0.2.5-custom.1 验收实例均未更新。日本生产未操作。
- 当前工作区基线仍是 0.2.0，本候选不包含其他工作区升级到 0.2.17 的全部改动。后续正式上线前需移植到正式版本基线。
- 已构建当前 tracked/untracked 非忽略工作区快照，包含此前已有改动；不是只构建 HEAD。未提交、推送、发布 tag 或 Release，也未运行本候选的 GitHub CI。
- 网络 `internal=true`，无公开映射端口；支付、主动监测、计划测试、自动备份和提示词审计关闭。真实支付、OAuth、外部邮件和上游模型调用不在本次可用验收范围。

## 制品与位置

| 项目 | 值 |
| --- | --- |
| 服务器 | `yinan@game.game-mp.cn:1022` |
| 根目录 | `/home/yinan/sub2api-staging/enterprise-20261021-DMQW2gg2` |
| Compose | 根目录下 `runtime/compose.json`，包含私密配置，不输出或提交 |
| Compose project | `sub2api-enterprise-cn` |
| 固定镜像 | `sub2api-custom:0.2.0-custom.3-candidate.enterprise.20261021` |
| 镜像 ID | `sha256:13e366e508102476a46a29d06bb033f0973f59d5c3092b5b5552226686ad3216` |
| 基线 HEAD | `53b12dc807efde190fcca71cd0501d259a78b652`，工作区有未提交修改 |
| 源码归档 SHA-256 | `f86cd91e348a985cc156cb70c3eeb5eb80a65120aafb72691917717a0058f9c6` |
| 二进制 SHA-256 | `34b4ed09d31fc739a9e001ad8427dd1ce05b6c984726c878ec34353f8d614d80` |
| 应用容器 | `sub2api-enterprise-cn-sub2api-1` |
| 应用 ID | `a53dfa05fe693eacaad1476f54a37b6530863c80c646177d80711488cbdb48b2` |
| 当前应用 IP | `172.27.0.4:8080` |
| 数据库容器 / DB | `sub2api-enterprise-cn-postgres-1` / `sub2api_enterprise` |
| 缓存容器 | `sub2api-enterprise-cn-redis-1` |

使用 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed`，嵌入本轮最终前端构建。运行基础镜像为已有 `sub2api-cn-runtime-base:20260918T022208Z`，核对 ID `sha256:87df7c0716c495cef1ce9ea369e3dd4f3fcb619e16f09bc46475ad116dd6da10`。源码归档不包含本部署记录。

## 数据与验证

- 从 `sub2api-stage-cn-postgres-1` 一致性备份，排除四张历史日志表的数据：`ops_system_logs`、`ops_error_logs`、`prompt_audit_events`、`prompt_audit_jobs`；保留表结构及业务数据。原数据库约 5.7 GB，恢复后副本约 508 MB。
- 备份为根目录下 `backup/staging-seed.dump`，约 57 MB；SHA-256 `ba87cdecf9a392e31c1f801004ac37b5c968ce4c90810c726139187f59c318e5`。`pg_restore --list` 及恢复到空实例成功。
- 仅在副本恢复旧基线所需的 `groups.models_list_config`，从 `codex_models_manifest_config` 初始化；未修改源库或迁移校验记录。企业迁移 `239_enterprise_accounts.sql` 自动执行成功。
- 数据库、Redis 密码及 JWT 独立生成；同步副本 `security_secrets.jwt_secret`，因为应用优先使用数据库持久化签名值。TOTP 加密配置沿用种子环境，已有密码未重置。
- 应用、PostgreSQL、Redis 均 healthy，主动重启应用后恢复，自动重启计数为 0。运行二进制哈希与上传制品一致。
- 30 项部署 smoke 检查通过：静态资源、登录、系统设置企业充值倍率默认 1、无月卡转换及余额保持、旧登录会话即时识别企业身份、普通/企业倍率独立保存、模型广场接口、活动限制 403、用户无法改回个人、创建 API 密钥、使用记录、渠道接口和月度 ZIP（三份 CSV、no-store）。
- 临时测试账号和 API 密钥均已软删除，保留正常审计和转换记录。非删除用户 151、账户 57、非删除密钥 158、订阅 49，与源库一致；原始密钥总数含软删除为 219。
- 既有两个验收项目的六个容器 ID 和镜像均未变化，全部 healthy。隧道首页、登录、用量页面均返回 200；旧 18081 健康检查正常。
- 重启后日志无 fatal、panic、迁移失败或校验和错误。外部价格/GitHub 同步因网络隔离失败属于预期限制。未进行真实付款、上游推理或浏览器人工验收。
- 部署后磁盘剩余约 3.9 GB；未清理其他项目、镜像或备份。候选日志已限制轮转大小。

证据存放于根目录 `artifacts/`：`source.tar.gz`、`existing-containers.json`、`smoke-results.json`、`final-verification.json` 和部署验证脚本。Dockerfile、二进制及兼容 SQL 在 `build/`。

## 隧道恢复与停止候选

Mac 重启或隧道退出后，确认 18082 空闲且容器 IP 未变，再运行：

```bash
ssh -F /dev/null -i /Users/varrge/.ssh/gamemulti_codex_deploy -p 1022 \
  -o BatchMode=yes -o ConnectTimeout=15 -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 -o ServerAliveCountMax=3 \
  -M -S /private/tmp/sub2api-cn-enterprise-DMQW2gg2.sock \
  -fNT -L 127.0.0.1:18082:172.27.0.4:8080 yinan@game.game-mp.cn
```

停止隧道：

```bash
ssh -F /dev/null -S /private/tmp/sub2api-cn-enterprise-DMQW2gg2.sock \
  -O exit -p 1022 yinan@game.game-mp.cn
```

停止候选时，仅在中国服务器对该 Compose 执行 `sudo docker compose -f /home/yinan/sub2api-staging/enterprise-20261021-DMQW2gg2/runtime/compose.json stop`，保留目录和数据库。无需回滚任何原有验收站，也不要把旧应用连接到已经发生企业转换的候选数据库。

## 2026-10-21 企业导航开关修正

- 用户追加要求：支付关闭则不显示充值，工单未开启则不显示我的工单。复用现有 `FeatureFlags.payment`、`FeatureFlags.supportTicket` 和导航过滤函数；未改变全站支付/工单配置或后端权限。
- 组件回归先复现 5 项失败，修正后企业导航、普通导航和功能路由共 50 项测试全部通过；Vue 类型检查、生产构建、Linux 嵌入构建通过。独立只读复核无重要问题。未重跑无关的后端全量测试或真实支付。
- 新固定镜像：`sub2api-custom:0.2.0-custom.3-candidate.enterprise.navfix.20261021`，ID `sha256:ec963bb4f437be2d3d4bf0cf915a75dc3442cf1d443ec5c025dafcb7500fa641`。
- 运行二进制 SHA-256：`77268abf454d667341de645300dccc59506fe2bf1c4a31b91b0052916e77cebd`。源码归档 SHA-256：`e6e0093e6663786db58d9ebf9d150ef9ec46fee968bb057e6f386e7c5e056ebf`。
- 仅以 `up -d --no-deps sub2api` 重建企业应用；新容器 ID `4c85c8acfb63dcaa9159faae4c8b8b6acd6849ab5d20e9aa0ed9646c87eadf46`。PostgreSQL 和 Redis ID 未变，无数据库迁移，临时体验账号保留且登录验证通过。
- 应用 healthy，自动重启 0；IP 仍为 `172.27.0.4`，原 18082 隧道继续可用。通过隧道下载的新 AppLayout 静态资源与本地构建逐字节一致。
- 站点当前 `payment_enabled=false`、`support_ticket_enabled=false`，刷新后企业菜单只有使用记录、渠道状态、API 密钥、个人资料。工单配置未加载时按现有 opt-in 规则隐藏；支付沿用普通用户的 opt-out 规则。
- 更新证据位于部署根目录 `updates/nav-20261021-VrYgORrE/`：源码、Dockerfile、二进制、`containers-before.json`、`verification.json`。旧配置保存在私密文件 `compose-before.private.json`，旧固定镜像仍保留；回滚只需将该配置恢复到 `runtime/compose.json` 后再次只重建企业应用，不恢复数据库。
- 部署后剩余磁盘约 2.9 GB；未清理现有镜像、备份或其他项目。

## 2026-10-21 方案 A 欢迎提示与特权设置

- 本轮实现边界见 [privileges.md](privileges.md)。管理员可独立设置企业欢迎开关、文案和预览；普通用户消费头衔待口径和权益确认，尚未自动授予，不改变计费和权限。
- UI 由实际 Kimi K3 YOLO 完成。97 项前端专项/相关回归、Go 三个包特权专项、Vue 类型检查、所增前端文件 ESLint、生产构建及 Linux 嵌入构建通过。独立复核提出的旧请求取消和 emoji 计数问题均已修复并复核关闭。
- 固定镜像 `sub2api-custom:0.2.0-custom.3-candidate.enterprise.welcome.20261021`；ID `sha256:4b053071e5de848c07dca0b45d9d4f8a74a5836e2b0edb087dc12c987b1332c0`。
- 二进制 SHA-256 `8322a43958bf3d5010c7d0e486843f5f2a3bfcf4a0fb7ea8b001710d469b08fd`；源码归档 SHA-256 `1dbe08240f443b3954a50cb5edc59296ce70d507ff54c3cebbaeba046e172d76`。归档含特权功能记录，不含本次部署完成后的记录追加；最后两行校验说明注释不影响已构建运行代码。
- 仅替换企业应用容器，新 ID `ad18370468d9d7cb9c53345176beaf544e87c26177b3da9b7031ff75f8a1a1e5`，仍为 `172.27.0.4:8080`；原 18082 隧道健康检查 200。数据库与 Redis 容器 ID 不变，无新增数据库迁移。另两个验收站六个容器的 ID 和镜像均未变，全部 healthy；日本生产未操作。
- 应用 healthy、自动重启 0、无宿主机公开端口。保留企业体验账号，实际登录、企业欢迎配置返回通过；企业用户读取管理员配置被拒绝（403）。支付与工单仍关闭。
- 正式页面浏览器验证通过：桌面、375px 手机、深色、减少动态效果；真实鼠标关闭、自动消失、站内切页不重播、刷新重播，无浏览器异常。80 emoji 手机布局另用浏览器模拟响应验证，无溢出，未修改服务器配置。一次仅 400ms 的入场阶段等待超时后单独重试通过；不作为产品故障结论。
- 未用真实管理员会话在部署站保存配置；管理员加载/保存/预览及校验由后端 handler 和真实前端组件测试覆盖。未执行真实支付、邮件、上游推理或全项目测试套件。
- 服务器证据在根目录 `updates/welcome-20261021-V8c2l0/`：源码、二进制、Dockerfile、更新/验证脚本、`containers-before.json`、`verification.json`。`compose-before.private.json` 和旧 navfix 镜像保留，回滚仍只需恢复该配置后 `up -d --no-deps sub2api`，不恢复数据库。
- 本机截图及浏览器验证脚本位于 `/tmp/sub2api-privileges-V8c2l0/`。部署后磁盘仅剩约 2.4 GB；未清理任何旧镜像或备份，后续部署前需关注空间。

## 2026-10-21 普通用户多条件头衔

- 普通用户与企业欢迎共用「管理员 → 系统设置 → 特权设置」。支持累计 API 消费、累计充值入账、注册时长、管理员指定用户四种条件；可配置头衔、欢迎文案、启停与优先级。首条匹配生效，默认规则为空，不自动授予现有用户。具体统计口径和可撤销性质见 [privileges.md](privileges.md)。
- UI 继续由实际 Kimi K3 YOLO 完成，业务逻辑由主代理实现。四个 Go 包专项、真实 PostgreSQL 集成、前端 103 项相关回归通过；最终布局调整后 42 项重测通过。类型检查、ESLint、前端和 Linux 嵌入构建通过，独立复核无未解决问题。
- 管理员页面的桌面/手机（375px）浏览器操作通过：四类型、手动 ID、排序、草稿删除、保存后重载、预览、头衔转义/清空、深色及无横向溢出；此项使用模拟 API。没有部署站真实管理员会话，因此未实际保存线上普通用户规则；后端保存和资格链路由 handler 与真实数据库测试覆盖。
- 固定镜像 `sub2api-custom:0.2.0-custom.3-candidate.enterprise.privileges.20261021`；ID `sha256:9f7c7caa045e50466766e271fc8d3f67a1a5653c0ef2127130506295ca7c3619`。
- 二进制 SHA-256 `8e75357df33d342ff73d6ff55cd8ce7a3b141d598fdf9d8c3592d05852e3a86f`；源码归档 SHA-256 `1b657944199a23c17bd3a296ad5926aeec897cda6fb7d8ab56cf7c5c2650762e`。归档不含本条部署完成记录。
- 只更新企业应用容器：ID `4dbddc2c458c5085f65b7b599bc47cbe3a9347c69c1e62a84ec23993ff221061`，IP 仍为 `172.27.0.4:8080`。应用 healthy、自动重启 0、无公开端口；PostgreSQL/Redis 及另外两个验收站的六个容器 ID/镜像未变，全部 healthy；无新增迁移，日本生产未操作。
- 原 18082 隧道断开，已按原监听地址和目标恢复，健康检查 200。保留企业体验账号，真实登录、服务端欢迎、no-store 和企业用户不能读取管理员配置（403）均通过；支付和工单仍关闭，余额/身份未修改。
- 新版正式企业页面桌面、手机、深色、减少动态效果浏览器回归通过；手动关闭、切页不重播、刷新重播、自动消失正常，企业用户不显示普通头衔。首次手机检查错过 400ms 入场窗口，调整测试等待覆盖入场/停留窗口后通过，产品代码未因此变更。
- 证据在服务器根目录 `updates/privileges-20261021-32zVoN20/`，包含制品、脚本、容器前后核对和 `verification.json`；旧配置 `compose-before.private.json` 与旧 welcome 固定镜像保留。回滚只恢复该配置后 `up -d --no-deps sub2api`，无需回滚数据库。
- 本机 UI 截图、浏览器脚本和 Kimi 报告位于 `/tmp/sub2api-personal-privileges-32zVoN20/`。未执行全项目测试、真实支付、邮件或上游模型调用。部署后磁盘约剩 1.9 GB（99% 已用），未清理任何其他项目、旧镜像或备份；继续部署前需清理或扩容。

## 2026-10-22 四种可选欢迎动效

- 同页选择 A 暖金胶囊、B 侧滑卡片、C 光环徽章、D 流光横幅；企业与每条普通用户规则独立保存。旧配置默认 A，旧客户端省略字段时保留已选效果，不因排序串到其他规则。具体语义见 [privileges.md](privileges.md)。未自动更改服务器的欢迎选择、普通用户规则或账号。
- UI/选择器由实际 Kimi K3 YOLO 制作，主代理负责配置、生命周期、页面绑定和测试。Go 特权专项、真实 PostgreSQL 特权集成、前端 9 文件 91 项、类型检查、ESLint、前端构建与 Linux 嵌入构建通过；独立复核发现的未知值选择边界已复现并修复，无剩余问题。
- 管理员页面 16 组浏览器检查通过：4 效果 × 桌面/深色手机/320px 长文案/减少动态效果；包括真实 transition 事件、一次性装饰动画、播放时无横向溢出、关闭按钮、键盘选择、独立预览/保存重载和未知值改回 A。管理员浏览器使用模拟 API，未使用测试服真实管理员会话保存配置。
- 固定镜像 `sub2api-custom:0.2.0-custom.3-candidate.enterprise.effects.20261022`；ID `sha256:44f22bfb015269a62b14dc0921bafbe2e928863c1913d9ed4e7523b8aa4afedb`。
- 二进制 SHA-256 `880c04d3b831b4d36f759a16cd8710bbda758b71b219a43f50214385919d8b62`；源码归档 SHA-256 `ccea6fbdea795991fa4c596bd187e86917cd03ab9b1eebd1dca604a3b9125951`。归档不含本次部署完成后的追加记录。
- 仅替换企业应用容器：ID `ac534ef267a21cb5ef73673587111bb2ecb3edaab24cb7881d5372186380b566`，IP `172.27.0.4:8080`，healthy、自动重启 0、无公开端口。PostgreSQL/Redis 与另两个验收站六个容器 ID/镜像均未变，全部 healthy；无新迁移，日本生产未操作。
- 18082 隧道已恢复并保持运行，健康检查 200。企业测试账号保留且真实登录通过；欢迎响应与数据库现有配置一致、no-store 正常、企业用户访问管理员配置仍 403。支付/工单保持关闭，头衔资格、身份、余额、倍率未改变。
- 部署后浏览器验证实际企业配置、关闭、切页不重播、刷新重播与自动消失通过；B/C/D 另使用浏览器内模拟欢迎响应验证正式构建资源，含手机、深色和减少动态效果，未写服务器配置。首次 C 页面导航等待超时；隧道健康与页面 HTTP 200，C/D 单独重试通过，未据此修改产品代码。
- 制品与验证证据位于服务器根目录 `updates/effects-20261022-4t8wZOek/`；旧 `compose-before.private.json` 与 privileges 固定镜像保留，回滚只恢复该配置并 `up -d --no-deps sub2api`，不回滚数据库。本机脚本和截图位于 `/tmp/sub2api-welcome-effects-4t8wZOek/`。
- 未执行全项目测试、真实支付、邮件、模型调用。部署后仅剩约 1.4 GB 可用空间，df 使用率四舍五入显示 100%；没有清理旧镜像、备份或其他项目，下次部署前必须先处理容量。
