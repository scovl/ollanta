-- Add optional warning threshold to gate conditions.
-- When set and violated (but error threshold is not), the condition enters WARN state.
ALTER TABLE gate_conditions
    ADD COLUMN IF NOT EXISTS warning_threshold NUMERIC;
