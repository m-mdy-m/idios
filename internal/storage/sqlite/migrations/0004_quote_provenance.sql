-- Preserve provenance for quotes so authored, verified quotations can carry their original source.
ALTER TABLE quotes ADD COLUMN source TEXT NOT NULL DEFAULT '';
ALTER TABLE quotes ADD COLUMN source_url TEXT NOT NULL DEFAULT '';
ALTER TABLE quotes ADD COLUMN category TEXT NOT NULL DEFAULT '';
