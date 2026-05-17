BEGIN;

DROP TABLE IF EXISTS medula_outgoing_message;
DROP TYPE IF EXISTS medula_outbox_status;
DROP TABLE IF EXISTS medula_provision;
DROP TYPE IF EXISTS medula_provision_status;
DROP TABLE IF EXISTS mernis_verification_log;

COMMIT;
