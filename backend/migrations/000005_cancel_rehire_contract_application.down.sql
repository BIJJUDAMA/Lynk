DROP INDEX IF EXISTS uq_contracts_active_application;

ALTER TABLE contracts
    ADD CONSTRAINT contracts_application_id_key UNIQUE (application_id);
