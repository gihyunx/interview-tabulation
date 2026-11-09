-- +goose Up
ALTER TABLE candidate ADD COLUMN number INT;

-- Existing candidates are numbered in the old name order.
UPDATE candidate c SET number = n.number
FROM (
    SELECT candidate_id, ROW_NUMBER() OVER (ORDER BY last_name, first_name, candidate_id) AS number
    FROM candidate
) n
WHERE c.candidate_id = n.candidate_id;

ALTER TABLE candidate
    ALTER COLUMN number SET NOT NULL,
    ADD CONSTRAINT candidate_number_key UNIQUE (number),
    ADD CONSTRAINT candidate_number_check CHECK (number > 0);

-- +goose Down
ALTER TABLE candidate DROP COLUMN number;
