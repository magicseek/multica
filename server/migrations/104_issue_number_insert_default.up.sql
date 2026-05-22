CREATE OR REPLACE FUNCTION assign_issue_number_if_missing()
RETURNS trigger AS $$
BEGIN
    IF NEW.number IS NULL OR NEW.number = 0 THEN
        UPDATE workspace
        SET issue_counter = GREATEST(
            issue_counter,
            (
                SELECT COALESCE(MAX(number), 0)
                FROM issue
                WHERE workspace_id = NEW.workspace_id
            )
        ) + 1
        WHERE id = NEW.workspace_id
        RETURNING issue_counter INTO NEW.number;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_assign_issue_number_if_missing ON issue;
CREATE TRIGGER trg_assign_issue_number_if_missing
BEFORE INSERT ON issue
FOR EACH ROW
EXECUTE FUNCTION assign_issue_number_if_missing();
