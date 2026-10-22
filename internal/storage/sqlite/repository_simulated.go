package sqlite

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"idios/internal/domain"
)

var simulatedTagName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,99}$`)

func cleanVirtualPath(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, `\`, `/`))
	if raw == "" || strings.HasPrefix(raw, "/") || strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("%w: file path must be a relative path inside the repository", domain.ErrInvalid)
	}
	cleaned := path.Clean(raw)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || cleaned == ".idios" || strings.HasPrefix(cleaned, ".idios/") {
		return "", fmt.Errorf("%w: file path cannot escape the repository", domain.ErrInvalid)
	}
	if len(cleaned) > 512 {
		return "", fmt.Errorf("%w: file path exceeds 512 characters", domain.ErrInvalid)
	}
	return cleaned, nil
}

func fileFrom(row Row) domain.RepositoryFile {
	return domain.RepositoryFile{ID: str(row, "id"), RepositoryID: str(row, "repository_id"), Path: str(row, "path"), Content: str(row, "content"), UpdatedAt: parseTime(row["updated_at"]), UpdatedInCompletionID: str(row, "updated_in_completion_id")}
}
func tagFrom(row Row) domain.RepositoryTag {
	return domain.RepositoryTag{ID: str(row, "id"), RepositoryID: str(row, "repository_id"), RepositoryName: str(row, "repository_name"), Name: str(row, "name"), Message: str(row, "message"), CompletionSequence: i64(row["completion_sequence"]), CompletionID: str(row, "completion_id"), PreviousTagID: str(row, "previous_tag_id"), PreviousTagName: str(row, "previous_tag_name"), CreatedAt: parseTime(row["created_at"]), FileCount: int(i64(row["file_count"]))}
}

const tagSelect = `SELECT t.id,t.repository_id,r.name AS repository_name,t.name,t.message,t.completion_sequence,t.completion_id,t.previous_tag_id,previous.name AS previous_tag_name,t.created_at,(SELECT COUNT(*) FROM repository_tag_files tf WHERE tf.tag_id=t.id) AS file_count FROM repository_tags t JOIN repositories r ON r.id=t.repository_id LEFT JOIN repository_tags previous ON previous.id=t.previous_tag_id`

func (s *Store) ListRepositoryFiles(ctx context.Context, repoRef string) ([]domain.RepositoryFile, error) {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, `SELECT id,repository_id,path,content,updated_at,updated_in_completion_id FROM repository_files WHERE repository_id=? ORDER BY lower(path),path`, repo.ID)
	if err != nil {
		return nil, err
	}
	return collect(rows, fileFrom), nil
}

func (s *Store) SaveRepositoryFile(ctx context.Context, repoRef, rawPath, content string) (domain.RepositoryFile, error) {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return domain.RepositoryFile{}, err
	}
	clean, err := cleanVirtualPath(rawPath)
	if err != nil {
		return domain.RepositoryFile{}, err
	}
	if len(content) > 2_000_000 {
		return domain.RepositoryFile{}, fmt.Errorf("%w: file content exceeds 2 MiB", domain.ErrInvalid)
	}
	now, id := nowText(), domain.NewID()
	err = s.run(ctx, func(p provider) error {
		paths, e := p.all(ctx, `SELECT path FROM repository_files WHERE repository_id=?`, repo.ID)
		if e != nil {
			return e
		}
		for _, row := range paths {
			existingPath := str(row, "path")
			if existingPath == clean {
				continue // updating the same file is allowed
			}
			if strings.HasPrefix(clean, existingPath+"/") || strings.HasPrefix(existingPath, clean+"/") {
				return fmt.Errorf("%w: file path %q overlaps with existing file %q", domain.ErrInvalid, clean, existingPath)
			}
		}
		existing, found, e := p.row(ctx, `SELECT id FROM repository_files WHERE repository_id=? AND path=?`, repo.ID, clean)
		if e != nil {
			return e
		}
		if found {
			id = str(existing, "id")
			_, e = p.changes(ctx, `UPDATE repository_files SET content=?,updated_at=? WHERE id=?`, content, now, id)
		} else {
			_, e = p.changes(ctx, `INSERT INTO repository_files(id,repository_id,path,content,updated_at) VALUES(?,?,?,?,?)`, id, repo.ID, clean, content, now)
		}
		if e != nil {
			return e
		}
		return activity(ctx, p, "repository_file", id, "repository_file.saved", marshalPayload(map[string]string{"repository": repo.Name, "path": clean}))
	})
	if err != nil {
		return domain.RepositoryFile{}, err
	}
	return fileFrom(Row{"id": id, "repository_id": repo.ID, "path": clean, "content": content, "updated_at": now}), nil
}

func (s *Store) DeleteRepositoryFile(ctx context.Context, repoRef, rawPath string) error {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return err
	}
	clean, err := cleanVirtualPath(rawPath)
	if err != nil {
		return err
	}
	return s.run(ctx, func(p provider) error {
		file, ok, e := p.row(ctx, `SELECT id FROM repository_files WHERE repository_id=? AND path=?`, repo.ID, clean)
		if e != nil {
			return e
		}
		if !ok {
			return fmt.Errorf("%w: file %q", domain.ErrNotFound, clean)
		}
		if _, e = p.changes(ctx, `DELETE FROM repository_files WHERE id=?`, str(file, "id")); e != nil {
			return e
		}
		return activity(ctx, p, "repository", "repository:"+repo.ID, "repository_file.deleted", marshalPayload(map[string]string{"repository": repo.Name, "path": clean}))
	})
}

func (s *Store) ListRepositoryTags(ctx context.Context, repoRef string) ([]domain.RepositoryTag, error) {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, tagSelect+` WHERE t.repository_id=? ORDER BY t.rowid DESC,t.name DESC`, repo.ID)
	if err != nil {
		return nil, err
	}
	return collect(rows, tagFrom), nil
}

func (s *Store) GetRepositoryTag(ctx context.Context, repoRef, tagRef string) (domain.RepositoryTag, error) {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return domain.RepositoryTag{}, err
	}
	row, err := s.p().must(ctx, "repository tag "+tagRef, tagSelect+` WHERE t.repository_id=? AND (t.id=? OR t.name=? COLLATE NOCASE) LIMIT 1`, repo.ID, tagRef, tagRef)
	if err != nil {
		return domain.RepositoryTag{}, err
	}
	return tagFrom(row), nil
}

func (s *Store) CreateRepositoryTag(ctx context.Context, repoRef, name, message string) (domain.RepositoryTag, error) {
	repo, err := s.GetRepository(ctx, repoRef)
	if err != nil {
		return domain.RepositoryTag{}, err
	}
	name = strings.TrimSpace(name)
	if !simulatedTagName.MatchString(name) {
		return domain.RepositoryTag{}, fmt.Errorf("%w: tag name must start with a letter or digit and contain only letters, digits, dot, underscore, plus or hyphen", domain.ErrInvalid)
	}
	if len(message) > 10000 {
		return domain.RepositoryTag{}, fmt.Errorf("%w: tag message is too long", domain.ErrInvalid)
	}
	branches, err := s.ListBranches(ctx, repo.ID)
	if err != nil {
		return domain.RepositoryTag{}, err
	}
	defaultBranch := ""
	for _, branch := range branches {
		if branch.IsDefault {
			defaultBranch = branch.ID
			break
		}
	}
	var tagID string
	now := nowText()
	err = s.run(ctx, func(p provider) error {
		latest, ok, e := p.row(ctx, `SELECT id,sequence,file_snapshot_ready FROM completions WHERE repository_id=? AND branch_id=? ORDER BY sequence DESC LIMIT 1`, repo.ID, defaultBranch)
		if e != nil {
			return e
		}
		seq := int64(0)
		var completion any
		if ok {
			seq = i64(latest["sequence"])
			completion = str(latest, "id")
		}
		previous, hasPrev, e := p.row(ctx, `SELECT id,completion_sequence FROM repository_tags WHERE repository_id=? ORDER BY rowid DESC LIMIT 1`, repo.ID)
		if e != nil {
			return e
		}
		var prev any
		prevSeq := int64(0)
		if hasPrev {
			prev = str(previous, "id")
			prevSeq = i64(previous["completion_sequence"])
		}
		tagID = domain.NewID()
		if _, e = p.changes(ctx, `INSERT INTO repository_tags(id,repository_id,name,message,completion_sequence,completion_id,previous_tag_id,created_at) VALUES(?,?,?,?,?,?,?,?)`, tagID, repo.ID, name, strings.TrimSpace(message), seq, completion, prev, now); e != nil {
			return e
		}
		// A release tag points at the latest simulated commit snapshot. Legacy completions
		// created before file snapshots existed fall back to the current virtual tree.
		if ok && boolRow(latest, "file_snapshot_ready") {
			// This is a new-format commit. Respect even an empty snapshot (all files deleted)
			// instead of accidentally including uncommitted working-tree changes.
			if _, e = p.changes(ctx, `INSERT INTO repository_tag_files(tag_id,path,content) SELECT ?,path,content FROM completion_files WHERE completion_id=?`, tagID, str(latest, "id")); e != nil {
				return e
			}
		} else {
			// Older completions predate file snapshots. Keep the current virtual tree available
			// for their first tag because no faithful historical file state can be reconstructed.
			if _, e = p.changes(ctx, `INSERT INTO repository_tag_files(tag_id,path,content) SELECT ?,path,content FROM repository_files WHERE repository_id=?`, tagID, repo.ID); e != nil {
				return e
			}
		}
		if seq > prevSeq {
			if _, e = p.changes(ctx, `INSERT INTO repository_tag_history(tag_id,completion_id) SELECT ?,id FROM completions WHERE repository_id=? AND branch_id=? AND sequence>? AND sequence<=?`, tagID, repo.ID, defaultBranch, prevSeq, seq); e != nil {
				return e
			}
		}
		return activity(ctx, p, "repository_tag", tagID, "repository_tag.created", marshalPayload(map[string]any{"repository": repo.Name, "tag": name, "sequence": seq, "previous_tag_id": prev}))
	})
	if err != nil {
		return domain.RepositoryTag{}, err
	}
	return s.GetRepositoryTag(ctx, repo.ID, tagID)
}

func (s *Store) GetRepositoryTagFiles(ctx context.Context, tagID string) ([]domain.RepositoryFile, error) {
	tag, err := s.p().must(ctx, "repository tag", `SELECT id,repository_id,created_at FROM repository_tags WHERE id=?`, tagID)
	if err != nil {
		return nil, err
	}
	rows, err := s.p().all(ctx, `SELECT '' AS id,? AS repository_id,path,content,? AS updated_at,'' AS updated_in_completion_id FROM repository_tag_files WHERE tag_id=? ORDER BY path`, str(tag, "repository_id"), str(tag, "created_at"), tagID)
	if err != nil {
		return nil, err
	}
	return collect(rows, fileFrom), nil
}

func (s *Store) CompareRepositoryTags(ctx context.Context, repoRef, tagRef string) ([]domain.TagFileChange, error) {
	tag, err := s.GetRepositoryTag(ctx, repoRef, tagRef)
	if err != nil {
		return nil, err
	}
	currentRows, err := s.p().all(ctx, `SELECT path,content FROM repository_tag_files WHERE tag_id=? ORDER BY path`, tag.ID)
	if err != nil {
		return nil, err
	}
	old := map[string]string{}
	if tag.PreviousTagID != "" {
		rows, e := s.p().all(ctx, `SELECT path,content FROM repository_tag_files WHERE tag_id=?`, tag.PreviousTagID)
		if e != nil {
			return nil, e
		}
		for _, r := range rows {
			old[str(r, "path")] = str(r, "content")
		}
	}
	result := make([]domain.TagFileChange, 0)
	for _, r := range currentRows {
		p, c := str(r, "path"), str(r, "content")
		prior, ok := old[p]
		if !ok {
			result = append(result, domain.TagFileChange{Path: p, Status: "added", Content: c})
		} else if prior != c {
			result = append(result, domain.TagFileChange{Path: p, Status: "modified", PreviousContent: prior, Content: c})
		}
		delete(old, p)
	}
	for p, c := range old {
		result = append(result, domain.TagFileChange{Path: p, Status: "deleted", PreviousContent: c})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}

func (s *Store) ListTagHistory(ctx context.Context, tagID string) ([]domain.CompletionRecord, error) {
	rows, err := s.p().all(ctx, completionSelect()+` JOIN repository_tag_history th ON th.completion_id=c.id WHERE th.tag_id=? ORDER BY c.sequence`, tagID)
	if err != nil {
		return nil, err
	}
	return collect(rows, completionFrom), nil
}
