-- Purchase documents are versioned separately from product/team quota rules.
CREATE TABLE month_card_rules (
 id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(id),
 draft_revision BIGINT NOT NULL DEFAULT 1,
 publication BIGINT NOT NULL DEFAULT 1,
 documents JSONB NOT NULL,
 published_documents JSONB NOT NULL,
 published_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE month_card_rule_publications (
 publication BIGINT PRIMARY KEY,
 documents JSONB NOT NULL,
 published_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE month_card_rule_reads (
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 document_id TEXT NOT NULL,
 version TEXT NOT NULL,
 read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(user_id,document_id,version)
);
INSERT INTO month_card_rules(id,documents,published_documents) VALUES(TRUE,'[{"id": "group-rules", "title": "拼团规则", "content": "1. 付款成功即获得独立月卡，不用等满团就能使用。每人独立持卡，额度不共享。\n\n2. 单独购买获得初始额度，不参加拼团升档；开团、参团按该团人数档位获得额度。\n\n3. 人数达档后增加额度，已用量不清零，重置时间和到期时间不变。\n\n4. 招募结束或取消后，已购月卡和已获得额度保留，不会自动退款。具体售价、额度和招募条件以购买页及对应团详情为准。", "active": true}, {"id": "purchase-notice", "title": "购买须知", "content": "1. 每次购买新增一张月卡，不给旧卡续期；付款成功后，累计可使用 30 天未冻结时间。\n\n2. 每 7 天未冻结时间重置周额度，周额度为总额度的 1/4，未用完的不结转；整卡累计使用不能超过总额度，最后一个周期随月卡到期结束。\n\n3. 冻结暂停计时，不重置额度，解冻后接着用。仅在平台开放冻结期间可操作，冻结期结束会自动解冻；冻结前已开始的调用照常扣费。\n\n4. 同分组月卡和订阅按设置顺序扣费；已开始调用的费用超出可用额度时，余额会补扣。余额欠费或没有可用权益时，暂停新调用。\n\n5. 月卡套餐不支持退款，订阅后不可退订，若因不可抗力因素无法供应，按比例原支付渠道退回。", "active": true}]'::jsonb,'[{"id": "group-rules", "title": "拼团规则", "content": "1. 付款成功即获得独立月卡，不用等满团就能使用。每人独立持卡，额度不共享。\n\n2. 单独购买获得初始额度，不参加拼团升档；开团、参团按该团人数档位获得额度。\n\n3. 人数达档后增加额度，已用量不清零，重置时间和到期时间不变。\n\n4. 招募结束或取消后，已购月卡和已获得额度保留，不会自动退款。具体售价、额度和招募条件以购买页及对应团详情为准。", "version": "40bc29cad8ba373f98df56af618889050f5b4af8d2bb69e7991bea92305ed6bc"}, {"id": "purchase-notice", "title": "购买须知", "content": "1. 每次购买新增一张月卡，不给旧卡续期；付款成功后，累计可使用 30 天未冻结时间。\n\n2. 每 7 天未冻结时间重置周额度，周额度为总额度的 1/4，未用完的不结转；整卡累计使用不能超过总额度，最后一个周期随月卡到期结束。\n\n3. 冻结暂停计时，不重置额度，解冻后接着用。仅在平台开放冻结期间可操作，冻结期结束会自动解冻；冻结前已开始的调用照常扣费。\n\n4. 同分组月卡和订阅按设置顺序扣费；已开始调用的费用超出可用额度时，余额会补扣。余额欠费或没有可用权益时，暂停新调用。\n\n5. 月卡套餐不支持退款，订阅后不可退订，若因不可抗力因素无法供应，按比例原支付渠道退回。", "version": "b34215a72f19931d6c1f8857599c31b516cc35ded6e2ef2a59502f48e2757415"}]'::jsonb);
INSERT INTO month_card_rule_publications(publication,documents,published_at)
 SELECT publication,published_documents,published_at FROM month_card_rules;
