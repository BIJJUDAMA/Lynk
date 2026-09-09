ALTER TABLE contracts ALTER COLUMN status SET DEFAULT 'active';

COMMENT ON COLUMN contracts.status IS 'active | completed | cancelled; draft is legacy and must not be newly inserted';
