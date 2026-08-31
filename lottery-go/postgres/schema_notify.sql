-- Optional: push sold-notifications to every replica so hints converge in
-- milliseconds instead of at the next refresh.
--
-- Apply this only when refresh latency is actually hurting. It adds a trigger
-- to the hot write path, which is a real cost.

CREATE OR REPLACE FUNCTION notify_ticket_sold() RETURNS trigger AS $$
BEGIN
    -- Fires only on the available/reserved -> sold transition, which is the
    -- only change that reduces the unsold count.
    IF NEW.status = 2 AND OLD.status <> 2 THEN
        PERFORM pg_notify('ticket_sold', NEW.num::text);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS tickets_sold_notify ON tickets;

CREATE TRIGGER tickets_sold_notify
    AFTER UPDATE OF status ON tickets
    FOR EACH ROW
    EXECUTE FUNCTION notify_ticket_sold();

-- Note: pg_notify payloads are capped (8000 bytes) and notifications are
-- delivered at COMMIT. If a transaction sells many tickets, the listener
-- receives one notification per row -- acceptable here because sales are
-- one-at-a-time, but worth knowing before reusing this pattern for bulk work.
