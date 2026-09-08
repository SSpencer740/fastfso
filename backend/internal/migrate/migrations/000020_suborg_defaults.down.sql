DROP TRIGGER IF EXISTS trg_user_default_suborg ON users;
DROP FUNCTION IF EXISTS add_user_to_default_suborg();

DROP TRIGGER IF EXISTS trg_tenant_default_suborg ON tenants;
DROP FUNCTION IF EXISTS create_default_suborg();
