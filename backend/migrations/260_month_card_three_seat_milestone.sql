-- Recruitment milestones survive later refunds, just like earned quota tiers.
ALTER TABLE month_card_teams ADD COLUMN three_seat_at TIMESTAMPTZ;
UPDATE month_card_teams t SET three_seat_at=NOW()
WHERE (SELECT count(*) FROM month_card_cards c JOIN payment_orders o ON o.id=c.order_id
    WHERE c.team_id=t.id AND (c.status<>'revoked' OR c.retain_team_seat)
    AND o.status<>'REFUNDED' AND o.paid_at<t.closes_at)>=3;
