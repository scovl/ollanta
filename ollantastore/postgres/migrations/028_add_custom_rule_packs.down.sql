ALTER TABLE scan_profile_snapshots DROP COLUMN IF EXISTS custom_catalog_hash;
DROP TABLE IF EXISTS custom_rule_audit;
DROP TABLE IF EXISTS custom_rule_versions;
DROP TABLE IF EXISTS custom_rule_packs;
