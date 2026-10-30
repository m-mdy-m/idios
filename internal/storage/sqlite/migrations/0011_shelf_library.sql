-- Shelf: local book catalog metadata, tags, cross-book knowledge relations and question archive.
-- Original README/PDF files stay in their existing directories; only filenames are persisted here.
CREATE TABLE IF NOT EXISTS shelf_books (
 id TEXT PRIMARY KEY,
 title TEXT NOT NULL,
 summary TEXT NOT NULL DEFAULT '',
 directory_name TEXT NOT NULL,
 readme_file TEXT NOT NULL DEFAULT '',
 pdf_file TEXT NOT NULL DEFAULT '',
 available INTEGER NOT NULL DEFAULT 1 CHECK(available IN (0,1)),
 reading_status TEXT NOT NULL DEFAULT 'unread' CHECK(reading_status IN ('unread','reading','finished')),
 current_page INTEGER NOT NULL DEFAULT 0 CHECK(current_page >= 0),
 total_pages INTEGER NOT NULL DEFAULT 0 CHECK(total_pages >= 0),
 repository_id TEXT REFERENCES repositories(id) ON DELETE SET NULL,
 last_scanned_at TEXT NOT NULL,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 CHECK(current_page <= total_pages OR total_pages = 0)
);
CREATE INDEX IF NOT EXISTS idx_shelf_books_available ON shelf_books(available,title COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_shelf_books_repository ON shelf_books(repository_id);

CREATE TABLE IF NOT EXISTS shelf_book_tags (
 book_id TEXT NOT NULL REFERENCES shelf_books(id) ON DELETE CASCADE,
 tag TEXT NOT NULL,
 normalized_tag TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(book_id, normalized_tag)
);
CREATE INDEX IF NOT EXISTS idx_shelf_book_tags_normalized ON shelf_book_tags(normalized_tag,book_id);

CREATE TABLE IF NOT EXISTS shelf_book_relations (
 id TEXT PRIMARY KEY,
 source_book_id TEXT NOT NULL REFERENCES shelf_books(id) ON DELETE CASCADE,
 target_book_id TEXT NOT NULL REFERENCES shelf_books(id) ON DELETE CASCADE,
 relation_type TEXT NOT NULL CHECK(relation_type IN ('related_to','prerequisite_for','contrasts_with','implements','references','uses')),
 created_at TEXT NOT NULL,
 CHECK(source_book_id <> target_book_id),
 UNIQUE(source_book_id,target_book_id,relation_type)
);
CREATE INDEX IF NOT EXISTS idx_shelf_relations_source ON shelf_book_relations(source_book_id);
CREATE INDEX IF NOT EXISTS idx_shelf_relations_target ON shelf_book_relations(target_book_id);

CREATE TABLE IF NOT EXISTS shelf_asks (
 id TEXT PRIMARY KEY,
 book_id TEXT NOT NULL REFERENCES shelf_books(id) ON DELETE CASCADE,
 question TEXT NOT NULL,
 context TEXT NOT NULL DEFAULT '',
 reason TEXT NOT NULL DEFAULT '',
 answer TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','answered')),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 answered_at TEXT,
 CHECK(length(trim(question)) > 0)
);
CREATE INDEX IF NOT EXISTS idx_shelf_asks_status ON shelf_asks(status,updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_shelf_asks_book ON shelf_asks(book_id,status,created_at DESC);
