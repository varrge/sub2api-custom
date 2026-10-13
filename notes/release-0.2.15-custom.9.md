# 0.2.15-custom.9

2026-10-13，用户完成中国测试服验收后授权合并并打 tag。发布分支为 `custom/0.2.15`，基于 `v0.2.15-custom.8` 合并 `fix/ops-entitlement-rejections`，其中包含周额度重置修正。

## 修正

- 手动清零月卡本周已用时，总已用同步扣除本周用量并最低归零。例如总已用 55、本周已用 30，重置后总已用 25、本周已用 0。总额度上限不变，自动换周沿用原行为，在途请求仍按原周期与代次结算。
- 明确的本地订阅不存在、过期或暂停仍返回 403，复用入口拒绝分钟汇总，避免逐条运维错误刷屏。续费后原 Key 恢复可用。
- 多分组全部被模型权限拒绝时返回 403 `MODEL_NOT_ALLOWED`；仅订阅不可用与模型拒绝的组合返回 403 `SUBSCRIPTION_NOT_FOUND`。后续可用分组正常回退，真实容量不足、存储故障和上游错误继续保留运维记录。

实现及验收记录：[周重置](month-card-weekly-reset-cn-deployment.md)、[业务拒绝规则](ops-business-rejections.md)、[中国验收](ops-business-rejections-cn-deployment.md)。相对 `.8` 无新增迁移。

## 发布前验证

- 后端全量 unit、integration 测试通过，集成测试连接本机隔离 PostgreSQL schema；月卡全量 PostgreSQL 测试与 race 检查通过。
- 前端 418 个测试文件、3242 项测试通过，ESLint 与类型检查通过。
- golangci-lint 2.14.0：0 issues。
- 部署脚本、发布工作流策略、发布矩阵辅助程序 10 项测试通过。
- 两项实现均完成独立代码审查；中国测试服 HTTP 验收与临时数据清理见上述部署记录。

本次合并和 tag 发布不升级服务器。中国仍运行已验收候选，正式镜像以 GitHub Release 工作流成功发布为准。
