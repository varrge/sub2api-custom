INSERT INTO settings(key,value) VALUES('activity_campaigns_enabled','false')
ON CONFLICT(key) DO NOTHING;

CREATE TABLE activity_campaigns (
    id TEXT PRIMARY KEY,
    revision BIGINT NOT NULL DEFAULT 1,
    config JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE activity_campaign_participants (
    campaign_id TEXT NOT NULL REFERENCES activity_campaigns(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    quiz_completed BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY(campaign_id,user_id)
);
CREATE TABLE activity_campaign_checkins (
    campaign_id TEXT NOT NULL,
    user_id BIGINT NOT NULL,
    day DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(campaign_id,user_id,day),
    FOREIGN KEY(campaign_id,user_id) REFERENCES activity_campaign_participants(campaign_id,user_id)
);
CREATE TABLE activity_campaign_rewards (
    campaign_id TEXT NOT NULL,
    user_id BIGINT NOT NULL,
    task TEXT NOT NULL CHECK(task IN ('first_checkin','three_checkins','quiz')),
    box_campaign_id TEXT NOT NULL,
    connection_id TEXT NOT NULL,
    earned_at TIMESTAMPTZ NOT NULL,
    delivered_at TIMESTAMPTZ,
    PRIMARY KEY(campaign_id,user_id,task),
    FOREIGN KEY(campaign_id,user_id) REFERENCES activity_campaign_participants(campaign_id,user_id)
);
INSERT INTO activity_campaigns(id,config) VALUES('double11-2026-wave1',
'{"enabled":false,"title":"双十一开门礼","subtitle":"签到答题，摇出三份好运","description":"首次签到、累计签到3天、完成趣味测验，各可领取1张普通摇摇卡。累计签到无需连续，答错可重试，每个账号最多获得3张。","starts_at":"2026-11-01T00:00:00+08:00","ends_at":"2026-11-06T00:00:00+08:00","box_campaign_id":"","questions":[{"id":"clock","prompt":"一天里，时针会在钟面上转几圈？","options":["1圈","2圈","24圈"],"answer":1},{"id":"keyboard","prompt":"下面哪个通常用来撤销上一步操作？","options":["Ctrl / Command + Z","空格键","Caps Lock"],"answer":0},{"id":"rainbow","prompt":"彩虹通常出现在什么方向？","options":["面向太阳","背向太阳","太阳正上方"],"answer":1}]}');
