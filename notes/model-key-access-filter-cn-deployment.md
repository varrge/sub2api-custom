# 按模型管理：仅显示支持的 Key 与分组

2026-09-23，按用户确认的范围修正右侧 Key 与分组显示，并沿用此前授权更新中国隔离测试实例。模型列表来源、手动输入方式及服务器模型配置保持原样。

## 行为与验证

- 只显示至少一个已绑定可用分组目录包含所选模型的 Key；多分组 Key 仅显示匹配分组。
- 不匹配的 Key 不参与计数、草稿和批量保存；搜索或分组筛选隐藏的符合条件 Key 仍包含在保存中。
- 目录加载失败时仅展示已确认匹配的 Key；无匹配来源的历史或手动模型仍可选，但右侧没有可编辑 Key。
- 前端 29 项相关测试、修改文件 ESLint、类型检查、生产构建与独立代码复查通过。
- 模拟接口浏览器覆盖桌面、手机、小屏、中英文和明暗主题；验证仅提交 11 把 Key 中的 6 把匹配 Key、冲突保留草稿及单 Key 双向同步。
- 部署后的真实浏览器逐一核对全部 70 个模型的 Key 行与分组标签，左侧模型清单保持一致；1440、390、320 像素无横向溢出或运行错误。真实验收阻止写请求，没有创建测试 Key 或修改现有 Key 权限。

## 部署与回滚

- 入口：<http://127.0.0.1:18081/keys/model-access>。
- 版本：`0.2.7-custom.11-candidate.key-filter.3b41fa444b8c`。
- 工作树 `sub2api-model-access`，分支 `feat/model-key-access`；基于 `74f39847a5d2792dfcb8fbaaea085bea4541f4e7` 的修复快照，逐文件哈希与源码归档锁定内容。
- 镜像 ID：`sha256:72b82b42d3bca2ae29461f5081a35dfc688ff6c2adaeb29da9df6ef2cba36346`。
- 二进制 SHA256：`0a4d7aca44e025df216244fd4d5105792ecde26587d82f23645b0d5f57d5bec8`。
- 源码归档 SHA256：`af52da2421848c7e618e022c78f07099401a018a34b4fe3b08acffd8abdabcbe`。
- 仅替换 `sub2api-multigroup-cn-sub2api-1`，约 10.9 秒恢复 healthy；最终重启次数 0。其他容器未重建，网络仍为 internal、无公网端口。
- 远端证据与备份：`/home/yinan/sub2api-staging/key-filter-20260923T151701Z`；本地证据：`/tmp/sub2api-key-filter-deploy-20260923T151701Z`。
- PostgreSQL dump 65,412,321 字节，通过 `pg_restore --list`；另保留应用数据和原 Compose。回滚镜像 `sub2api-restore:key-filter-20260923t151701z`，回滚仅切换应用、不恢复覆盖数据库。
- 用户、账号、Key、订阅记录数量均与部署前一致；无新增迁移。更新后可用磁盘约 858 MiB。

保留原 `v0.2.7-custom.11` 标签；本次修复未新建或移动标签，未部署日本生产。
