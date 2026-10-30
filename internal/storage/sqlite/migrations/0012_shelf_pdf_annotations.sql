-- Persistent PDF reading annotations: selections, highlight colors, and marginal notes.
CREATE TABLE IF NOT EXISTS shelf_annotations (
 id TEXT PRIMARY KEY,
 book_id TEXT NOT NULL REFERENCES shelf_books(id) ON DELETE CASCADE,
 page INTEGER NOT NULL CHECK(page > 0),
 quote TEXT NOT NULL DEFAULT '',
 note TEXT NOT NULL DEFAULT '',
 color TEXT NOT NULL DEFAULT 'yellow' CHECK(color IN ('yellow','green','blue','pink','purple')),
 rectangles_json TEXT NOT NULL DEFAULT '[]',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_shelf_annotations_book_page ON shelf_annotations(book_id,page,created_at);
