-- Allow a new active contract to reuse an application_id after a prior contract was cancelled.
-- Unconditional UNIQUE contracts_application_id_key (from 000001) blocked rehire.
ALTER TABLE contracts DROP CONSTRAINT IF EXISTS contracts_application_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS uq_contracts_active_application
ON contracts(application_id)
WHERE status NOT IN ('cancelled');

-- uq_contracts_active_job already exists from 000004; do not recreate.
