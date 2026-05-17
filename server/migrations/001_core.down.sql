BEGIN;

DROP TRIGGER IF EXISTS trg_branch_assignment_updated_at ON branch_assignment;
DROP TRIGGER IF EXISTS trg_role_updated_at ON role;
DROP TRIGGER IF EXISTS trg_org_membership_updated_at ON org_membership;
DROP TRIGGER IF EXISTS trg_app_user_updated_at ON app_user;
DROP TRIGGER IF EXISTS trg_department_updated_at ON department;
DROP TRIGGER IF EXISTS trg_branch_updated_at ON branch;
DROP TRIGGER IF EXISTS trg_organization_updated_at ON organization;

DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS branch_assignment;
DROP TABLE IF EXISTS role_permission;
DROP TABLE IF EXISTS role;
DROP TABLE IF EXISTS permission;
DROP TABLE IF EXISTS org_membership;
DROP TABLE IF EXISTS user_session;
DROP TABLE IF EXISTS app_user;
DROP TABLE IF EXISTS department;
DROP TABLE IF EXISTS branch;
DROP TABLE IF EXISTS organization;

COMMIT;
