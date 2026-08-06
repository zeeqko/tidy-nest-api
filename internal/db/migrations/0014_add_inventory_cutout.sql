-- Background-removed copy of an item's photo, generated once at add-item
-- time (see PLAN.md T1). Kept alongside imageURL, not instead of it:
-- inventory surfaces keep showing the original photo while outfit surfaces
-- fall back to imageURL when cutoutURL is empty, so every pre-existing item
-- keeps working with zero backfill.
ALTER TABLE Inventories ADD COLUMN cutoutURL TEXT;
