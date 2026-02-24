# 客户数据接入 Checklist｜企业微信标签对接字段清单

## 目标
- 在意向金到账后 24 小时内完成最小可用数据接入，确保 3 套 SOP 可直接运行。

## A. 基础信息（必填）
- [ ] `external_user_id`（企业微信外部联系人唯一ID）
- [ ] `name`（客户称呼）
- [ ] `phone`（可选，脱敏存储）
- [ ] `source_channel`（线索来源）
- [ ] `owner_user_id`（归属员工）
- [ ] `created_at`（建联时间）

## B. 标签体系（必填）
- [ ] `lifecycle_stage`（潜客/成交/在服/续费/流失）
- [ ] `intent_level`（高/中/低）
- [ ] `industry`（教育/零售/本地生活/ToB等）
- [ ] `customer_tier`（A/B/C）
- [ ] `risk_tag`（投诉风险/沉默风险/续费风险）
- [ ] `scenario_tag`（售后回访/续费提醒/沉默激活）

## C. 行为与时间字段（SOP触发核心）
- [ ] `last_interaction_at`（最近互动时间）
- [ ] `last_followup_time`（最近跟进时间）
- [ ] `expire_date`（服务到期日，续费场景必填）
- [ ] `deal_time`（成交时间，售后场景必填）
- [ ] `next_action_time`（下次触达时间）

## D. 交易与结果字段（回写必填）
- [ ] `order_amount`（订单金额）
- [ ] `payment_status`（未支付/部分/已支付）
- [ ] `renewal_intent`（高/中/低/未知）
- [ ] `followup_status`（已触达/已回复/待人工）
- [ ] `reactivation_result`（已激活/暂不需要/无响应）
- [ ] `case_outcome`（成功/失败/跟进中）

## E. 合规与质检字段（必填）
- [ ] `consent_status`（是否同意接收消息）
- [ ] `sensitive_flag`（是否命中敏感词）
- [ ] `complaint_flag`（是否投诉）
- [ ] `do_not_disturb_until`（免打扰截止时间）
- [ ] `qa_score`（质检评分）

## F. 接口与权限检查
- [ ] 企业微信通讯录与外部联系人接口权限已开通
- [ ] 标签读写权限已配置（机器人/应用）
- [ ] 消息发送权限与频控策略已验证
- [ ] 回写接口可用（CRM/表格/数据库）
- [ ] 日志审计可追溯（触发、发送、回写全链路）

## G. 启动前验收（Go-Live Gate）
- [ ] 三套SOP触发条件已通过沙盒测试
- [ ] 每套SOP至少完成1次“触发->执行->回写”闭环演练
- [ ] 人工兜底SLA负责人已明确（姓名+值班时段）
- [ ] 合规词库与禁用词已加载
- [ ] 首日看板已可查看（SLA、回复率、预约率、人工接管率）

## 字段映射建议（企业微信 -> 交付系统）
- 企业微信客户ID -> `external_user_id`
- 企业微信标签 -> `scenario_tag` / `risk_tag` / `intent_level`
- 最近会话时间 -> `last_interaction_at`
- 群/私聊会话结果 -> `followup_status` / `reactivation_result`
- 商机状态 -> `renewal_intent` / `payment_status`

## 决策说明（执行侧）
- 采用“最小字段集优先”策略：先跑通 24 小时交付闭环，再扩展高级画像字段。
- 若客户系统字段缺失：以 `external_user_id + scenario_tag + last_interaction_at` 作为最低可运行集先上线，缺失字段在 T+3 天补齐。

