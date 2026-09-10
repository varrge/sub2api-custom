# 前端实现报告

## 已实现

- `/group-buy`：真实团列表、双列/单列卡片、分组筛选、团号/链接查询、团号及链接复制、倒计时与精确截止时间、当前每卡额度、下一档与满团人数分别展示、已参团/满团/过期禁用加入、历史团详情。
- `/purchase?tab=subscription`：页签“拼团”，真实商品基础额/周额/档位/招募期与独购、开团入口。参团通过 `mode=join&team_code=...`，使用团快照价格。旧 `tab=subscription&group=...` 定位新商品分组，不自动续期；原生旧套餐仍通过明确展开及选择购买，充值未改动。
- 共用支付链路：`month_card` 请求、CNY 限制、真实手续费及错误、二维码/Stripe/Airwallex/微信恢复路径、solo/create/join 参数跨 OAuth 保留、过期团禁止提交。支付结果查询本次订单的卡与团，提供团号复制与订阅入口。
- `/subscriptions`：按后台消耗顺序混排独立卡与旧订阅；每卡号、团号、状态、剩余日数、精确获得/到期时间、周/总用量和剩余额度、精确周周期以及最后周期提示。旧订阅原额度与重置逻辑保持。
- 同分组排序支持拖动、上下按钮及取消/保存，提交后台返回的完整有效权益集合；周额度耗尽的卡保留顺序。顶部摘要使用同一顺序及每卡实际上限。
- 调用扣费按 `request_id + api_key_id` 汇总完整请求，显示独立卡/旧订阅份额、余额差额和合计；卡与旧订阅的同数值 ID 严格区分。负余额提示及充值入口。
- `/admin/group-buy`：商品/人数档位/分组绑定/价格/招募期/上下架编辑、团快照与成员订单查询、按用户查询卡与分摊明细。团成员来自独立 `/admin/group-buy/teams/:code/cards`。卡详情连接现有订单查询及人工退款，全单 CNY 退款，撤卡影响提示。
- 普通用户及管理员个人导航、大厅/管理路由鉴权、中英文文案；未修改 `/custom/:id` 内嵌销售。

## 验证

- 11 项新月卡关键单元/组件测试已通过：人数门槛、重复参团、关闭/满团/过期、第五周期、独立卡/旧订阅排序、组间隔离、真实费用守恒、同数值 ID 身份隔离。
- 3 项大厅测试已通过：实时列表过滤、原团分享链接、支付路由、加载失败与空状态区分。
- 支付视图 22 项测试通过，包含新增 solo/create 固定价格、非 CNY 禁用、三种模式 OAuth 恢复；微信回调追加三种模式测试。
- 管理退款测试新增全额人民币/不可部分退款/无余额扣减开关断言。
- 完整前端测试：`npx --yes pnpm@9 --dir frontend test:run`，267 个测试文件、1920 项测试全部通过。
- 全量静态检查：`npx --yes pnpm@9 --dir frontend lint:check` 通过；`git diff --check -- frontend` 通过。
- 类型检查：`npx --yes pnpm@9 --dir frontend typecheck` 通过。
- 最终生产构建：`npx --yes pnpm@9 --dir frontend build` 通过（Vite 1074 modules，18.79s）。已有 Browserslist 数据较旧与大资源块提示，不影响构建。
- 完整日志：`/tmp/groupbuy-frontend-tests-final.log`、`/tmp/groupbuy-typecheck-final.log`、`/tmp/groupbuy-final-build.log`。

## 边界

- 没有配置或运行真实支付渠道付款；前端验证使用接口模拟数据，产品运行时均调用真实接口。
- 主代理追加本地 Chromium 浏览器检查，使用模拟接口数据打开真实 Vite 页面：大厅、我的订阅、拼团购买页在 1440×1000 与 390×844 下均无水平溢出和 JavaScript 异常。截图位于 `/tmp/sub2api-monthcard-{hall,cards,purchase}-{desktop,mobile}.png`；此检查不代表真实渠道支付或后端网络联调。
- 团详情展示实际成员、当前档位、固定档位规则和结束状态。接口未提供逐次档位事件历史，因此未虚构历史事件。

## 审核补充：已核实退款的故障恢复

- 后台订单列表与详情增加“核实已退款”入口。仅支持已尝试整单退款的月卡：`refund_amount === amount`、非空 `refund_reason`，状态为 `REFUND_PENDING`、`REFUND_FAILED`、更新时间超过五分钟的 `REFUNDING`，或 `COMPLETED`（按后端最新约定，涵盖人工退款通信失败后恢复为已完成的订单）。普通已完成订单不显示该入口。
- 对话框明确记录渠道已完成退款并撤销关联月卡，不会再次发起渠道退款，也不充值余额。必须填写去首尾空白后的 3–500 UTF-8 字节渠道流水或核实说明，并显式勾选已核实全额退款。
- 唯一提交端点为 `POST /admin/payment/orders/:id/refund/confirm`，载荷 `{reference, confirmed:true}`。重复点击被阻止；服务端冲突/错误保留对话框并展示错误，只有 `success:true` 才关闭并刷新订单。
- 新增状态/字节边界、真实对话框、订单列表接入与 API 契约测试；加上现有退款金额和国际化回归检查，共 6 个文件、39 项测试通过。
- 本次 `typecheck`、全量 `lint:check`、`git diff --check -- frontend` 和生产 `build` 均通过（Vite 14.06s）。
- 未进行实际退款或任何外部平台操作。验证日志位于 `/tmp/month-card-refund-ui-tests.log`、`/tmp/month-card-refund-ui-typecheck.log`、`/tmp/month-card-refund-ui-lint.log`、`/tmp/month-card-refund-ui-build.log`。
- 追加退款核实入口后再次执行完整前端测试：270 个测试文件、1946 项测试全部通过，日志 `/tmp/sub2api-monthcard-frontend-final.log`。
