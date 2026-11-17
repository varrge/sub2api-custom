-- Refunds can be retried after a verified failure. Each attempt needs its own
-- immutable audit; uniqueness for recharge and affiliate claims stays intact.
DROP INDEX IF EXISTS idx_payment_audit_logs_order_action_uniq;
CREATE UNIQUE INDEX idx_payment_audit_logs_order_action_uniq
    ON payment_audit_logs(order_id, action)
    WHERE action NOT LIKE 'REFUND_%';
