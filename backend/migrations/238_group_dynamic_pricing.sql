-- Dynamic group pricing keeps groups.rate_multiplier as the single effective
-- billing value while deriving it from the highest remote procurement rate:
--
--   effective rate = MAX(remote_group_multiplier) + markup
--
-- Account health/schedulability is deliberately not part of the aggregate. A
-- temporarily unhealthy account can recover and re-enter scheduling, so removing
-- it from the price floor would create a loss window.

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS dynamic_pricing_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS dynamic_pricing_markup DECIMAL(10,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS manual_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS dynamic_source_max_multiplier DECIMAL(10,4),
    ADD COLUMN IF NOT EXISTS dynamic_pricing_updated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS dynamic_pricing_status VARCHAR(20) NOT NULL DEFAULT 'manual';

-- Existing groups remain static and keep their current price as the fallback.
UPDATE groups
   SET manual_rate_multiplier = rate_multiplier,
       dynamic_pricing_status = 'manual'
 WHERE dynamic_pricing_enabled = FALSE;

ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_dynamic_pricing_markup_nonnegative;
ALTER TABLE groups ADD CONSTRAINT groups_dynamic_pricing_markup_nonnegative
    CHECK (dynamic_pricing_markup >= 0);

ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_manual_rate_multiplier_positive;
ALTER TABLE groups ADD CONSTRAINT groups_manual_rate_multiplier_positive
    CHECK (manual_rate_multiplier > 0);

ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_dynamic_pricing_status_valid;
ALTER TABLE groups ADD CONSTRAINT groups_dynamic_pricing_status_valid
    CHECK (dynamic_pricing_status IN ('manual', 'ready', 'no_accounts', 'error'));

CREATE OR REPLACE FUNCTION refresh_group_dynamic_pricing(p_group_id BIGINT)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
    pricing_enabled BOOLEAN;
    fallback_rate NUMERIC(10,4);
    markup NUMERIC(10,4);
    source_max NUMERIC(10,4);
    account_count BIGINT;
    next_rate NUMERIC(10,4);
    next_status VARCHAR(20);
    affected INTEGER;
BEGIN
    -- Separate lock and aggregate statements are intentional. Under READ COMMITTED,
    -- the aggregate gets a fresh snapshot after a concurrent refresher releases the
    -- group row, so a later low-cost update cannot overwrite a newer high-cost floor.
    PERFORM id
      FROM groups
     WHERE id = p_group_id AND deleted_at IS NULL
     FOR UPDATE;
    IF NOT FOUND THEN
        RETURN FALSE;
    END IF;

    SELECT dynamic_pricing_enabled, manual_rate_multiplier, dynamic_pricing_markup
      INTO pricing_enabled, fallback_rate, markup
      FROM groups
     WHERE id = p_group_id;

    IF NOT pricing_enabled THEN
        RETURN FALSE;
    END IF;

    SELECT COUNT(*) FILTER (WHERE a.id IS NOT NULL),
           MAX(a.remote_group_multiplier) FILTER (
               WHERE a.remote_group_multiplier IS NOT NULL
                 AND a.remote_group_multiplier > 0
           )
      INTO account_count, source_max
      FROM account_groups ag
      JOIN accounts a ON a.id = ag.account_id
     WHERE ag.group_id = p_group_id
       AND a.deleted_at IS NULL;

    IF account_count = 0 THEN
        next_rate := fallback_rate;
        next_status := 'no_accounts';
    ELSIF source_max IS NULL THEN
        -- A local accounts.rate_multiplier is a downstream billing setting,
        -- not a remote procurement price. Never use it as a silent fallback;
        -- expose the missing remote data and keep the configured safe fallback.
        next_rate := fallback_rate;
        next_status := 'error';
    ELSE
        next_rate := ROUND(source_max + markup, 4);
        next_status := 'ready';
    END IF;

    UPDATE groups
       SET rate_multiplier = next_rate,
           dynamic_source_max_multiplier = source_max,
           dynamic_pricing_status = next_status,
           dynamic_pricing_updated_at = NOW(),
           updated_at = NOW()
     WHERE id = p_group_id
       AND (rate_multiplier,
            dynamic_source_max_multiplier,
            dynamic_pricing_status)
           IS DISTINCT FROM
           (next_rate, source_max, next_status);
    GET DIAGNOSTICS affected = ROW_COUNT;
    RETURN affected > 0;
END;
$$;

CREATE OR REPLACE FUNCTION refresh_dynamic_pricing_after_account_group_insert()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE target_group_id BIGINT;
BEGIN
    FOR target_group_id IN SELECT DISTINCT group_id FROM new_account_groups LOOP
        PERFORM refresh_group_dynamic_pricing(target_group_id);
    END LOOP;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION refresh_dynamic_pricing_after_account_group_delete()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE target_group_id BIGINT;
BEGIN
    FOR target_group_id IN SELECT DISTINCT group_id FROM old_account_groups LOOP
        PERFORM refresh_group_dynamic_pricing(target_group_id);
    END LOOP;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION refresh_dynamic_pricing_after_account_group_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE target_group_id BIGINT;
BEGIN
    FOR target_group_id IN
        SELECT group_id FROM new_account_groups
        UNION
        SELECT group_id FROM old_account_groups
    LOOP
        PERFORM refresh_group_dynamic_pricing(target_group_id);
    END LOOP;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_account_groups_dynamic_pricing_insert ON account_groups;
CREATE TRIGGER trg_account_groups_dynamic_pricing_insert
AFTER INSERT ON account_groups
REFERENCING NEW TABLE AS new_account_groups
FOR EACH STATEMENT EXECUTE FUNCTION refresh_dynamic_pricing_after_account_group_insert();

DROP TRIGGER IF EXISTS trg_account_groups_dynamic_pricing_delete ON account_groups;
CREATE TRIGGER trg_account_groups_dynamic_pricing_delete
AFTER DELETE ON account_groups
REFERENCING OLD TABLE AS old_account_groups
FOR EACH STATEMENT EXECUTE FUNCTION refresh_dynamic_pricing_after_account_group_delete();

DROP TRIGGER IF EXISTS trg_account_groups_dynamic_pricing_update ON account_groups;
CREATE TRIGGER trg_account_groups_dynamic_pricing_update
AFTER UPDATE ON account_groups
REFERENCING OLD TABLE AS old_account_groups NEW TABLE AS new_account_groups
FOR EACH STATEMENT EXECUTE FUNCTION refresh_dynamic_pricing_after_account_group_update();

CREATE OR REPLACE FUNCTION refresh_dynamic_pricing_after_account_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE target_group_id BIGINT;
BEGIN
    FOR target_group_id IN
        SELECT DISTINCT ag.group_id
          FROM account_groups ag
          JOIN new_accounts n ON n.id = ag.account_id
          JOIN old_accounts o ON o.id = n.id
         WHERE o.remote_group_multiplier IS DISTINCT FROM n.remote_group_multiplier
            OR o.deleted_at IS DISTINCT FROM n.deleted_at
    LOOP
        PERFORM refresh_group_dynamic_pricing(target_group_id);
    END LOOP;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_accounts_dynamic_pricing_update ON accounts;
CREATE TRIGGER trg_accounts_dynamic_pricing_update
AFTER UPDATE ON accounts
REFERENCING OLD TABLE AS old_accounts NEW TABLE AS new_accounts
FOR EACH STATEMENT EXECUTE FUNCTION refresh_dynamic_pricing_after_account_update();

CREATE OR REPLACE FUNCTION refresh_dynamic_pricing_after_group_config()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.dynamic_pricing_enabled THEN
            PERFORM refresh_group_dynamic_pricing(NEW.id);
        END IF;
        RETURN NULL;
    END IF;

    IF OLD.dynamic_pricing_enabled IS NOT DISTINCT FROM NEW.dynamic_pricing_enabled
       AND OLD.dynamic_pricing_markup IS NOT DISTINCT FROM NEW.dynamic_pricing_markup
       AND OLD.manual_rate_multiplier IS NOT DISTINCT FROM NEW.manual_rate_multiplier THEN
        RETURN NULL;
    END IF;

    IF NEW.dynamic_pricing_enabled THEN
        PERFORM refresh_group_dynamic_pricing(NEW.id);
    ELSE
        UPDATE groups
           SET rate_multiplier = NEW.manual_rate_multiplier,
               dynamic_source_max_multiplier = NULL,
               dynamic_pricing_updated_at = NULL,
               dynamic_pricing_status = 'manual',
               updated_at = NOW()
         WHERE id = NEW.id
           AND (rate_multiplier,
                dynamic_source_max_multiplier,
                dynamic_pricing_updated_at,
                dynamic_pricing_status)
               IS DISTINCT FROM
               (NEW.manual_rate_multiplier, NULL, NULL, 'manual');
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_groups_dynamic_pricing_insert ON groups;
CREATE TRIGGER trg_groups_dynamic_pricing_insert
AFTER INSERT ON groups
FOR EACH ROW EXECUTE FUNCTION refresh_dynamic_pricing_after_group_config();

DROP TRIGGER IF EXISTS trg_groups_dynamic_pricing_config_update ON groups;
CREATE TRIGGER trg_groups_dynamic_pricing_config_update
AFTER UPDATE OF dynamic_pricing_enabled, dynamic_pricing_markup, manual_rate_multiplier ON groups
FOR EACH ROW EXECUTE FUNCTION refresh_dynamic_pricing_after_group_config();

-- The durable auth-cache invalidation trigger must also notice a dynamic-mode
-- change when the effective numeric rate happens to stay the same.
CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_group_id BIGINT;
BEGIN
    target_group_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.is_exclusive IS NOT DISTINCT FROM NEW.is_exclusive
       AND OLD.allow_image_generation IS NOT DISTINCT FROM NEW.allow_image_generation
       AND OLD.platform IS NOT DISTINCT FROM NEW.platform
       AND OLD.subscription_type IS NOT DISTINCT FROM NEW.subscription_type
       AND OLD.rate_multiplier IS NOT DISTINCT FROM NEW.rate_multiplier
       AND OLD.dynamic_pricing_enabled IS NOT DISTINCT FROM NEW.dynamic_pricing_enabled
       AND OLD.peak_rate_enabled IS NOT DISTINCT FROM NEW.peak_rate_enabled
       AND OLD.peak_start IS NOT DISTINCT FROM NEW.peak_start
       AND OLD.peak_end IS NOT DISTINCT FROM NEW.peak_end
       AND OLD.peak_rate_multiplier IS NOT DISTINCT FROM NEW.peak_rate_multiplier
       AND OLD.profit_control_enabled IS NOT DISTINCT FROM NEW.profit_control_enabled
       AND OLD.profit_min_margin IS NOT DISTINCT FROM NEW.profit_min_margin
       AND OLD.profit_safety_buffer IS NOT DISTINCT FROM NEW.profit_safety_buffer
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
      FROM api_keys AS k
     WHERE k.group_id = target_group_id
       AND k.deleted_at IS NULL
       AND k.key <> '';
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

-- Reconcile any rows enabled by hand before/while this migration was applied.
DO $$
DECLARE target_group_id BIGINT;
BEGIN
    FOR target_group_id IN
        SELECT id FROM groups WHERE dynamic_pricing_enabled = TRUE AND deleted_at IS NULL
    LOOP
        PERFORM refresh_group_dynamic_pricing(target_group_id);
    END LOOP;
END;
$$;
