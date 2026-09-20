# v0.2.7 中国候选部署记录

日期：2026-09-20。仅部署中国隔离测试环境；未创建 tag、Release 或 GHCR 正式版本，未操作日本生产。

## 版本与验证

- 官方基线：`v0.2.7` / `aea725f2ea644d5592d0bbb1d63b607efa7e200a`。
- 分支：`feature/merge-upstream-0.2.7`。
- 部署源码：`8bfcb5e4ca35d4e5c14fbb55ca949393e0a05b99`。
- 页面版本：`0.2.7-custom.1-candidate.8bfcb5e4c`。
- 固定候选镜像：`sub2api-custom:0.2.7-custom.1-candidate.8bfcb5e4c`。
- 候选镜像 ID：`sha256:9927cb9fe68323c99d4b04323c30534f2195a87296f6e94ef156bf11173be9c8`。
- 应用二进制 SHA-256：`d87570a93e97a7187bf6fe58e03ebf4ab2de7e2d621e252f2696fa205b4d829f`。
- 源码归档 SHA-256：`6d7fa4688009fd7df4d510cc801ef30663ab705fb54903479cba0895f8407d4f`。
- 镜像构建上下文 SHA-256：`c3565677bbb59acbd40b7073b7ce30fc0545967eb535a35ec656f5f5260648bf`。
- 同一源码提交的 [GitHub CI](https://github.com/varrge/sub2api-custom/actions/runs/35505624265) 四项全部成功。
- 本地后端单元/集成、全量 golangci-lint、真实 PostgreSQL 月卡并发与恢复测试通过；前端 331 文件 / 2462 测试、类型检查、Lint、构建和浏览器检查通过。
- 独立复审通过，未定价 Seedance 零费用结算问题已修复。

## 中国环境

- Compose project：`sub2api-multigroup-cn`。只重建应用，PostgreSQL、Redis 及所有记录的其他项目容器 ID 不变。
- 应用容器 ID：`970040a25a423a9aa4aacecdf70e028def634defa12ed8847f2ed185dfed1562`。
- 切换至 healthy：7.7 秒（未单独测量每次 HTTP 中断时长）。
- 应用重启后 healthy：6.7 秒，应用容器 ID 不变。
- 业务网络保持 `internal=true`，没有发布宿主机端口。
- 本机 SSH 隧道：`http://127.0.0.1:18081`，会话 socket `/tmp/sub2api-cn-glass-20260920.sock`。
- API 验收通过：月卡商品/拼团/权益、优惠码鉴权、密钥分组和模型列表、条件计划创建/编辑/省略字段保留/删除。测试计划始终关闭，未触发探测。
- 真实浏览器桌面和手机检查通过：月卡页面正常显示，模型允许/禁用切换、条件测试开关可见可操作，无 JavaScript 错误；浏览器未保存对现有密钥/计划的修改。
- 未执行真实供应商调用；隔离环境无法验证 Ark 实际视频生成。

## 数据与回滚

| 表 | 切换前后总数 | 切换前后未删除数 |
| --- | ---: | ---: |
| users | 154 | 151 |
| accounts | 57 | 19 |
| api_keys | 225 | 158 |
| user_subscriptions | 49 | 24 |

- 备份目录：`/home/yinan/sub2api-staging/upstream027-20260920T101502Z/backup`。
- 精确回滚镜像：`sub2api-restore:upstream027-20260920t101502z`。
- PostgreSQL dump：65,810,629 字节；SHA-256 `7b42252428a14c230cf5c32ee8b1f2e4ad138b2b8e83f7baa3c52d0244457951`。
- 已验证 PostgreSQL 归档目录、Redis RDB、运行二进制与回滚镜像一致、全备份校验和。
- 保留原始私有 Compose、应用数据和运行镜像归档。凭据不记录在本文。
- 回滚只重建应用，不恢复数据库。旧镜像支持模型禁用规则，但不识别条件测试；回滚脚本会先停用已启用的条件测试计划并保存 ID，不把条件字段改成 false。

兼容实现与决策见 [合并说明](upstream-0.2.7-integration.md)。
