-- Which Stylebook tab a category appears under. NULL = hidden from Stylebook.
-- Backfill mirrors model.DefaultStylebookGroup (same rule order).
ALTER TABLE Categories ADD COLUMN stylebookGroup TEXT
    CHECK (stylebookGroup IN ('clothes', 'shoes', 'accessories', 'bags'));

UPDATE Categories SET stylebookGroup = CASE
    WHEN name ILIKE '%shoe%' OR name ILIKE '%footwear%' THEN 'shoes'
    WHEN name ILIKE '%accessor%' THEN 'accessories'
    WHEN name ILIKE '%bag%' THEN 'bags'
    WHEN name ILIKE '%cloth%' OR name ILIKE '%top%' OR name ILIKE '%bottom%'
        OR name ILIKE '%apparel%' OR name ILIKE '%wear%' THEN 'clothes'
    ELSE NULL
END;
