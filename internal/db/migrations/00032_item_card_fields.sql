-- +goose Up
-- What an item card shows beyond its name and price (rebuild Task 1c,
-- ADR-147): a short description, a dietary mark, a gluten-free flag and a
-- popular flag. All optional, and every existing item has none of them.
--
-- diet is one of veg, non_veg, vegan, or '' for no mark. Gluten free is its
-- own flag, not a fifth mark: a dish can be vegetarian and gluten free, and a
-- single column could not say both. The description is a card's one line, so
-- it is held to 140 characters here as well as in the editor.

ALTER TABLE items ADD COLUMN description TEXT NOT NULL DEFAULT ''
    CHECK (length(description) <= 140);
ALTER TABLE items ADD COLUMN diet TEXT NOT NULL DEFAULT ''
    CHECK (diet IN ('', 'veg', 'non_veg', 'vegan'));
ALTER TABLE items ADD COLUMN gluten_free BOOLEAN NOT NULL DEFAULT 0
    CHECK (gluten_free IN (0, 1));
ALTER TABLE items ADD COLUMN popular BOOLEAN NOT NULL DEFAULT 0
    CHECK (popular IN (0, 1));

-- +goose Down
ALTER TABLE items DROP COLUMN popular;
ALTER TABLE items DROP COLUMN gluten_free;
ALTER TABLE items DROP COLUMN diet;
ALTER TABLE items DROP COLUMN description;
