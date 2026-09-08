-- Trigger: automatically create a "Default" sub-org when a tenant is created.
CREATE FUNCTION create_default_suborg()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO suborganizations (tenant_id, name) VALUES (NEW.id, 'Default');
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_tenant_default_suborg
    AFTER INSERT ON tenants
    FOR EACH ROW EXECUTE FUNCTION create_default_suborg();

-- Trigger: automatically add a new user to their tenant's Default sub-org.
CREATE FUNCTION add_user_to_default_suborg()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO user_suborganizations (user_id, suborganization_id)
    SELECT NEW.id, s.id
    FROM suborganizations s
    WHERE s.tenant_id = NEW.tenant_id AND s.name = 'Default'
    ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_user_default_suborg
    AFTER INSERT ON users
    FOR EACH ROW EXECUTE FUNCTION add_user_to_default_suborg();

-- Backfill: create a Default sub-org for every existing tenant that has none.
INSERT INTO suborganizations (tenant_id, name)
SELECT t.id, 'Default'
FROM tenants t
WHERE t.deleted_at IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM suborganizations s WHERE s.tenant_id = t.id
  );

-- Backfill: assign every user not yet in any sub-org to their tenant's Default.
INSERT INTO user_suborganizations (user_id, suborganization_id)
SELECT u.id, s.id
FROM users u
JOIN suborganizations s ON s.tenant_id = u.tenant_id AND s.name = 'Default'
WHERE NOT EXISTS (
    SELECT 1 FROM user_suborganizations us WHERE us.user_id = u.id
);
