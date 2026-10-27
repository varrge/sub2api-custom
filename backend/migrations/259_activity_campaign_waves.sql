ALTER TABLE activity_campaign_rewards DROP CONSTRAINT activity_campaign_rewards_task_check;
ALTER TABLE activity_campaign_rewards
    ADD CONSTRAINT activity_campaign_rewards_task_check CHECK (
      (campaign_id='double11-2026-wave1' AND task IN ('first_checkin','three_checkins','quiz')) OR
      (campaign_id='double11-2026-wave2' AND task ~ '^(purchase|team)_[1-9][0-9]{0,18}$') OR
      (campaign_id='double11-2026-wave3' AND task IN ('treasure_three','treasure_five'))
    ),
    ADD COLUMN card_tier TEXT NOT NULL DEFAULT 'low' CHECK(card_tier IN ('low','medium')),
    ADD COLUMN quantity INTEGER NOT NULL DEFAULT 1 CHECK(quantity BETWEEN 1 AND 2),
    ADD COLUMN order_id BIGINT REFERENCES payment_orders(id);

CREATE TABLE activity_campaign_purchases (
    campaign_id TEXT NOT NULL REFERENCES activity_campaigns(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    slot SMALLINT NOT NULL CHECK(slot BETWEEN 1 AND 3),
    order_id BIGINT NOT NULL REFERENCES payment_orders(id),
    card_id BIGINT NOT NULL REFERENCES month_card_cards(id),
    price_cny INTEGER NOT NULL CHECK(price_cny IN (58,128,198)),
    paid_at TIMESTAMPTZ NOT NULL,
    team_qualified_at TIMESTAMPTZ,
    PRIMARY KEY(campaign_id,user_id,slot),
    UNIQUE(campaign_id,order_id),
    FOREIGN KEY(campaign_id,user_id) REFERENCES activity_campaign_participants(campaign_id,user_id)
);
CREATE TABLE activity_campaign_treasures (
    campaign_id TEXT NOT NULL REFERENCES activity_campaigns(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    treasure_id TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    token_expires_at TIMESTAMPTZ NOT NULL,
    found_at TIMESTAMPTZ,
    PRIMARY KEY(campaign_id,user_id,treasure_id),
    FOREIGN KEY(campaign_id,user_id) REFERENCES activity_campaign_participants(campaign_id,user_id)
);
CREATE INDEX activity_campaign_pending_rewards
    ON activity_campaign_rewards(campaign_id,user_id,earned_at) WHERE delivered_at IS NULL;

INSERT INTO activity_campaigns(id,config) VALUES
('double11-2026-wave2',
'{"enabled":false,"title":"月卡拼团加赠","subtitle":"购买有礼，三人满团再加码","description":"58月卡购买赠1张普通卡、满团再赠1张；128月卡购买赠2张普通卡、满团再赠1张；198月卡购买赠1张中级卡、满团再赠2张普通卡。每账号前3张符合条件的月卡参与，支持自拼。活动内购买，招募期内满团，活动结束后也可追加。","starts_at":"2026-11-06T00:00:00+08:00","ends_at":"2026-11-12T00:00:00+08:00","box_campaign_id":"","questions":[]}'),
('double11-2026-wave3',
'{"enabled":false,"title":"全站寻宝","subtitle":"找到幸运小鲸鱼，解锁双份好礼","description":"5只幸运小鲸鱼藏在站内页面，找到后点击即可保存进度。找到任意3只获得1张普通摇摇卡，集齐5只再得1张。无需购买，线索分日揭晓，请在活动结束前完成寻找并领取奖励。","starts_at":"2026-11-12T00:00:00+08:00","ends_at":"2026-11-16T00:00:00+08:00","box_campaign_id":"","questions":[]}');
