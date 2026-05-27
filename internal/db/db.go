package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pdfrg/must/internal/config"
	"github.com/pdfrg/must/internal/genre"
	"github.com/pdfrg/must/internal/models"

	_ "modernc.org/sqlite"
)

var logger *log.Logger

func SetLogger(l *log.Logger) {
	logger = l
}

type LibraryDB struct {
	db *sql.DB
}

func NewLibraryDB() (*LibraryDB, error) {
	dbPath := config.GetLibraryDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ld := &LibraryDB{db: db}
	if err := ld.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return ld, nil
}

func (ld *LibraryDB) Close() error {
	return ld.db.Close()
}

func (ld *LibraryDB) initSchema() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := ld.db.Exec(p); err != nil {
			return fmt.Errorf("failed to set pragma %q: %w", p, err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS tracks (
		id INTEGER PRIMARY KEY,
		path TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL DEFAULT '',
		artist TEXT NOT NULL DEFAULT '',
		album TEXT NOT NULL DEFAULT '',
		album_artist TEXT NOT NULL DEFAULT '',
		year INTEGER NOT NULL DEFAULT 0,
		genre TEXT NOT NULL DEFAULT '',
		track_num INTEGER NOT NULL DEFAULT 0,
		disc_num INTEGER NOT NULL DEFAULT 0,
		duration REAL NOT NULL DEFAULT 0,
		has_cover_art INTEGER NOT NULL DEFAULT 0,
		file_mod_time INTEGER NOT NULL DEFAULT 0
	);

CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist);
CREATE INDEX IF NOT EXISTS idx_tracks_album_artist ON tracks(album_artist);
CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album);
CREATE INDEX IF NOT EXISTS idx_tracks_path ON tracks(path);

	CREATE VIRTUAL TABLE IF NOT EXISTS tracks_fts USING fts5(
		title,
		artist,
		album,
		genre,
		content='tracks',
		content_rowid='id',
		tokenize='porter unicode61',
		prefix='2 3'
	);

	CREATE TRIGGER IF NOT EXISTS tracks_ai AFTER INSERT ON tracks BEGIN
		INSERT INTO tracks_fts(rowid, title, artist, album, genre)
		VALUES (new.id, new.title, new.artist, new.album, new.genre);
	END;

	CREATE TRIGGER IF NOT EXISTS tracks_ad AFTER DELETE ON tracks BEGIN
		INSERT INTO tracks_fts(tracks_fts, rowid, title, artist, album, genre)
		VALUES ('delete', old.id, old.title, old.artist, old.album, old.genre);
	END;

	CREATE TRIGGER IF NOT EXISTS tracks_au AFTER UPDATE ON tracks BEGIN
		INSERT INTO tracks_fts(tracks_fts, rowid, title, artist, album, genre)
		VALUES ('delete', old.id, old.title, old.artist, old.album, old.genre);
		INSERT INTO tracks_fts(rowid, title, artist, album, genre)
		VALUES (new.id, new.title, new.artist, new.album, new.genre);
	END;
	`

	if _, err := ld.db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

func (ld *LibraryDB) InsertTrack(t *models.Track) (int64, error) {
	result, err := ld.db.Exec(`
	INSERT OR REPLACE INTO tracks
		(path, title, artist, album, album_artist, year, genre,
		track_num, disc_num, duration, has_cover_art, file_mod_time)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Path, t.Title, t.Artist, t.Album, t.AlbumArtist,
		t.Year, t.Genre, t.TrackNum, t.DiscNum, t.Duration,
		t.HasCoverArt, t.FileModTime,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert track: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}
	t.ID = id
	return id, nil
}

func (ld *LibraryDB) InsertTracksBatch(tracks []*models.Track) (int, error) {
	if len(tracks) == 0 {
		return 0, nil
	}

	tx, err := ld.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.Prepare(`
	INSERT OR REPLACE INTO tracks
		(path, title, artist, album, album_artist, year, genre,
		track_num, disc_num, duration, has_cover_art, file_mod_time)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	inserted := 0
	for _, t := range tracks {
		result, err := stmt.Exec(
			t.Path, t.Title, t.Artist, t.Album, t.AlbumArtist,
			t.Year, t.Genre, t.TrackNum, t.DiscNum, t.Duration,
			t.HasCoverArt, t.FileModTime,
		)
		if err != nil {
			if logger != nil {
				logger.Printf("Error inserting %s: %v", t.Path, err)
			}
			continue
		}
		id, _ := result.LastInsertId()
		t.ID = id
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return inserted, nil
}

func (ld *LibraryDB) GetTrackByPath(path string) (*models.Track, error) {
	row := ld.db.QueryRow(`
		SELECT id, path, title, artist, album, album_artist, year, genre,
		       track_num, disc_num, duration, has_cover_art, file_mod_time
		FROM tracks WHERE path = ?`, path)

	var t models.Track
	var hasCover int
	err := row.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album,
		&t.AlbumArtist, &t.Year, &t.Genre, &t.TrackNum, &t.DiscNum,
		&t.Duration, &hasCover, &t.FileModTime)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get track: %w", err)
	}
	t.HasCoverArt = hasCover != 0
	return &t, nil
}

func (ld *LibraryDB) GetAllArtists() ([]string, error) {
	rows, err := ld.db.Query(`SELECT DISTINCT COALESCE(NULLIF(album_artist, ''), artist) FROM tracks WHERE COALESCE(NULLIF(album_artist, ''), artist) != '' ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("failed to query artists: %w", err)
	}
	defer func() { _ = rows.Close() }()

	seen := make(map[string]bool)
	var artists []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		clean := stripFeat(a)
		if clean != "" && !seen[clean] {
			seen[clean] = true
			artists = append(artists, clean)
		}
	}
	return artists, rows.Err()
}

func stripFeat(name string) string {
	for _, sep := range []string{" feat.", " feat ", " ft.", " ft ", " featuring ", " Featuring "} {
		if idx := strings.Index(strings.ToLower(name), sep); idx > 0 {
			return strings.TrimSpace(name[:idx])
		}
	}
	return name
}

func (ld *LibraryDB) GetAlbumsByArtist(artist string) ([]string, error) {
	rows, err := ld.db.Query(`SELECT DISTINCT album FROM tracks WHERE (COALESCE(NULLIF(album_artist, ''), artist) = ? OR COALESCE(NULLIF(album_artist, ''), artist) LIKE ?) AND album != '' ORDER BY album`, artist, artist+" feat.%")
	if err != nil {
		return nil, fmt.Errorf("failed to query albums: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var albums []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (ld *LibraryDB) GetTracksByArtistAndAlbum(artist, album string) ([]models.Track, error) {
	rows, err := ld.db.Query(`
	SELECT id, path, title, artist, album, album_artist, year, genre,
	track_num, disc_num, duration, has_cover_art, file_mod_time
	FROM tracks
	WHERE (COALESCE(NULLIF(album_artist, ''), artist) = ? OR COALESCE(NULLIF(album_artist, ''), artist) LIKE ?) AND album = ?
	ORDER BY disc_num, track_num`, artist, artist+" feat.%", album)
	if err != nil {
		return nil, fmt.Errorf("failed to query tracks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTracks(rows)
}

func (ld *LibraryDB) GetTracksByArtist(artist string) ([]models.Track, error) {
	rows, err := ld.db.Query(`
	SELECT id, path, title, artist, album, album_artist, year, genre,
	track_num, disc_num, duration, has_cover_art, file_mod_time
	FROM tracks
	WHERE COALESCE(NULLIF(album_artist, ''), artist) = ? OR COALESCE(NULLIF(album_artist, ''), artist) LIKE ?
	ORDER BY album, disc_num, track_num`, artist, artist+" feat.%")
	if err != nil {
		return nil, fmt.Errorf("failed to query tracks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTracks(rows)
}

func (ld *LibraryDB) GetGenres() ([]string, error) {
	rows, err := ld.db.Query(`SELECT DISTINCT genre FROM tracks WHERE genre != ''`)
	if err != nil {
		return nil, fmt.Errorf("failed to query genres: %w", err)
	}
	defer func() { _ = rows.Close() }()

	seen := make(map[string]bool)
	var genres []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		for _, part := range splitGenre(g) {
			part = strings.TrimSpace(part)
			if part != "" && !seen[part] {
				seen[part] = true
				genres = append(genres, part)
			}
		}
	}
	slices.Sort(genres)
	return genres, rows.Err()
}

func splitGenre(g string) []string {
	return genre.Split(g)
}

func (ld *LibraryDB) GetAlbumsByGenre(genre string) ([]string, error) {
	rows, err := ld.db.Query(`SELECT DISTINCT COALESCE(NULLIF(album_artist, ''), artist), album FROM tracks WHERE (genre = ? OR genre LIKE ? OR genre LIKE ? OR genre LIKE ?) AND album != '' ORDER BY album`, genre, genre+";%", "%; "+genre, "%; "+genre+";%")
	if err != nil {
		return nil, fmt.Errorf("failed to query albums by genre: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var albums []string
	for rows.Next() {
		var artist, album string
		if err := rows.Scan(&artist, &album); err != nil {
			return nil, err
		}
		albums = append(albums, artist+" - "+album)
	}
	return albums, rows.Err()
}

func (ld *LibraryDB) GetTracksByAlbum(album string) ([]models.Track, error) {
	rows, err := ld.db.Query(`
		SELECT id, path, title, artist, album, album_artist, year, genre,
		track_num, disc_num, duration, has_cover_art, file_mod_time
		FROM tracks
		WHERE album = ?
		ORDER BY disc_num, track_num`, album)
	if err != nil {
		return nil, fmt.Errorf("failed to query tracks by album: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTracks(rows)
}

func (ld *LibraryDB) GetAllTracks() ([]models.Track, error) {
	rows, err := ld.db.Query(`
		SELECT id, path, title, artist, album, album_artist, year, genre,
		       track_num, disc_num, duration, has_cover_art, file_mod_time
		FROM tracks
		ORDER BY artist, album, disc_num, track_num`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all tracks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTracks(rows)
}

func (ld *LibraryDB) GetAllAlbums() ([]string, error) {
	rows, err := ld.db.Query(`SELECT DISTINCT album FROM tracks WHERE album != '' ORDER BY album`)
	if err != nil {
		return nil, fmt.Errorf("failed to query albums: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var albums []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func albumOrderClause(sort string) string {
	switch sort {
	case config.SortYearDesc:
		return `ORDER BY CASE WHEN album_year = 0 THEN 1 ELSE 0 END, album_year DESC, album`
	case config.SortYearAsc:
		return `ORDER BY CASE WHEN album_year = 0 THEN 1 ELSE 0 END, album_year ASC, album`
	default:
		return `ORDER BY album`
	}
}

func (ld *LibraryDB) GetAlbumsByArtistSorted(artist, sort string) ([]models.AlbumEntry, error) {
	orderBy := albumOrderClause(sort)
	query := fmt.Sprintf(`SELECT album, COALESCE(MAX(NULLIF(year, 0)), 0) AS album_year
		FROM tracks
		WHERE (COALESCE(NULLIF(album_artist, ''), artist) = ? OR COALESCE(NULLIF(album_artist, ''), artist) LIKE ?)
		AND album != '' GROUP BY album %s`, orderBy)
	rows, err := ld.db.Query(query, artist, artist+" feat.%")
	if err != nil {
		return nil, fmt.Errorf("failed to query albums: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAlbumEntries(rows)
}

func (ld *LibraryDB) GetAlbumsByGenreSorted(genre, sort string) ([]models.AlbumEntry, error) {
	orderBy := albumOrderClause(sort)
	query := fmt.Sprintf(`SELECT COALESCE(NULLIF(album_artist, ''), artist) || ' - ' || album,
		COALESCE(MAX(NULLIF(year, 0)), 0) AS album_year
		FROM tracks
		WHERE (genre = ? OR genre LIKE ? OR genre LIKE ? OR genre LIKE ?)
		AND album != '' GROUP BY COALESCE(NULLIF(album_artist, ''), artist), album %s`, orderBy)
	rows, err := ld.db.Query(query, genre, genre+";%", "%; "+genre, "%; "+genre+";%")
	if err != nil {
		return nil, fmt.Errorf("failed to query albums by genre: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAlbumEntries(rows)
}

func (ld *LibraryDB) GetAllAlbumsSorted(sort string) ([]models.AlbumEntry, error) {
	orderBy := albumOrderClause(sort)
	query := fmt.Sprintf(`SELECT album, COALESCE(MAX(NULLIF(year, 0)), 0) AS album_year
		FROM tracks
		WHERE album != '' GROUP BY album %s`, orderBy)
	rows, err := ld.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query albums: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAlbumEntries(rows)
}

func scanAlbumEntries(rows *sql.Rows) ([]models.AlbumEntry, error) {
	var entries []models.AlbumEntry
	for rows.Next() {
		var e models.AlbumEntry
		if err := rows.Scan(&e.Name, &e.Year); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (ld *LibraryDB) GetTracksByField(field string, values []string) ([]models.Track, error) {
	if len(values) == 0 {
		return nil, nil
	}

	var whereClause string
	var args []any

	switch field {
	case "artist":
		placeholders := make([]string, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args = append(args, v)
		}
		whereClause = fmt.Sprintf(`COALESCE(NULLIF(album_artist, ''), artist) IN (%s)`, strings.Join(placeholders, ","))
	case "album":
		placeholders := make([]string, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args = append(args, v)
		}
		whereClause = fmt.Sprintf(`album IN (%s)`, strings.Join(placeholders, ","))
	case "title":
		placeholders := make([]string, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args = append(args, v)
		}
		whereClause = fmt.Sprintf(`title IN (%s)`, strings.Join(placeholders, ","))
	case "genre":
		var orParts []string
		for _, v := range values {
			orParts = append(orParts, `(genre = ? OR genre LIKE ? OR genre LIKE ? OR genre LIKE ?)`)
			args = append(args, v, v+";%", "%; "+v, "%; "+v+";%")
		}
		whereClause = strings.Join(orParts, " OR ")
	default:
		return nil, fmt.Errorf("unknown field: %s", field)
	}

	query := fmt.Sprintf(`
		SELECT id, path, title, artist, album, album_artist, year, genre,
			track_num, disc_num, duration, has_cover_art, file_mod_time
		FROM tracks
		WHERE %s
		ORDER BY artist, album, disc_num, track_num
		LIMIT 500`, whereClause)

	rows, err := ld.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tracks by %s: %w", field, err)
	}
	defer func() { _ = rows.Close() }()

	return scanTracks(rows)
}

func (ld *LibraryDB) SearchFTS(query string) ([]models.Track, error) {
	return ld.SearchWithYearRange(query, 0, 0)
}

func (ld *LibraryDB) SearchWithYearRange(ftsQuery string, yearMin, yearMax int) ([]models.Track, error) {
	if ftsQuery == "" && yearMin == 0 && yearMax == 0 {
		return nil, nil
	}

	var whereClauses []string
	var args []any

	if ftsQuery != "" {
		whereClauses = append(whereClauses, "tracks_fts MATCH ?")
		args = append(args, ftsQuery)
	}

	if yearMin > 0 && yearMax > 0 {
		whereClauses = append(whereClauses, "t.year BETWEEN ? AND ?")
		args = append(args, yearMin, yearMax)
	} else if yearMin > 0 {
		whereClauses = append(whereClauses, "t.year >= ?")
		args = append(args, yearMin)
	} else if yearMax > 0 {
		whereClauses = append(whereClauses, "t.year <= ?")
		args = append(args, yearMax)
	}

	if len(whereClauses) == 0 {
		return nil, nil
	}

	whereStr := strings.Join(whereClauses, " AND ")

	query := fmt.Sprintf(`
		SELECT t.id, t.path, t.title, t.artist, t.album, t.album_artist,
			t.year, t.genre, t.track_num, t.disc_num, t.duration,
			t.has_cover_art, t.file_mod_time
		FROM tracks t
		JOIN tracks_fts fts ON t.id = fts.rowid
		WHERE %s
		ORDER BY bm25(tracks_fts, 10, 20, 5, 1)
		LIMIT 100`, whereStr)

	rows, err := ld.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTracks(rows)
}

func (ld *LibraryDB) DeleteTrackByPath(path string) error {
	_, err := ld.db.Exec(`DELETE FROM tracks WHERE path = ?`, path)
	return err
}

func (ld *LibraryDB) DeleteMissingTracks(existingPaths map[string]bool) (int, error) {
	result, err := ld.db.Exec(`DELETE FROM tracks WHERE path NOT IN (`+
		buildPathList(len(existingPaths))+`)`, mapToSlice(existingPaths)...)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (ld *LibraryDB) TrackCount() (int, error) {
	var count int
	err := ld.db.QueryRow(`SELECT COUNT(*) FROM tracks`).Scan(&count)
	return count, err
}

func (ld *LibraryDB) ResetZeroDurationModTimes() (int, error) {
	result, err := ld.db.Exec(`UPDATE tracks SET file_mod_time = 0 WHERE duration = 0`)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (ld *LibraryDB) GetTrackByID(id int64) (*models.Track, error) {
	row := ld.db.QueryRow(`
		SELECT id, path, title, artist, album, album_artist, year, genre,
		       track_num, disc_num, duration, has_cover_art, file_mod_time
		FROM tracks WHERE id = ?`, id)

	var t models.Track
	var hasCover int
	err := row.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album,
		&t.AlbumArtist, &t.Year, &t.Genre, &t.TrackNum, &t.DiscNum,
		&t.Duration, &hasCover, &t.FileModTime)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get track by id: %w", err)
	}
	t.HasCoverArt = hasCover != 0
	return &t, nil
}

func (ld *LibraryDB) SearchArtistsLike(query string) ([]string, error) {
	pattern := "%" + query + "%"
	rows, err := ld.db.Query(`
		SELECT DISTINCT COALESCE(NULLIF(album_artist, ''), artist)
		FROM tracks
		WHERE COALESCE(NULLIF(album_artist, ''), artist) LIKE ?
		ORDER BY 1 LIMIT 50`, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search artists: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var artists []string
	seen := make(map[string]bool)
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		if !seen[a] {
			seen[a] = true
			artists = append(artists, a)
		}
	}
	return artists, rows.Err()
}

type AlbumSearchResult struct {
	Album, Artist string
}

func (ld *LibraryDB) SearchAlbumsLike(query string) ([]AlbumSearchResult, error) {
	pattern := "%" + query + "%"
	rows, err := ld.db.Query(`
		SELECT DISTINCT album, COALESCE(NULLIF(album_artist, ''), artist)
		FROM tracks
		WHERE album LIKE ? AND album != ''
		ORDER BY album LIMIT 50`, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search albums: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var albums []AlbumSearchResult
	seen := make(map[string]bool)
	for rows.Next() {
		var a, ar string
		if err := rows.Scan(&a, &ar); err != nil {
			return nil, err
		}
		key := ar + "|" + a
		if !seen[key] {
			seen[key] = true
			albums = append(albums, AlbumSearchResult{Album: a, Artist: ar})
		}
	}
	return albums, rows.Err()
}

func (ld *LibraryDB) TotalDuration() (float64, error) {
	var total float64
	err := ld.db.QueryRow(`SELECT COALESCE(SUM(duration), 0) FROM tracks`).Scan(&total)
	return total, err
}

func (ld *LibraryDB) SearchLike(query string) ([]models.Track, error) {
	pattern := "%" + query + "%"
	rows, err := ld.db.Query(`
		SELECT id, path, title, artist, album, album_artist, year, genre,
			track_num, disc_num, duration, has_cover_art, file_mod_time
		FROM tracks
		WHERE title LIKE ? OR artist LIKE ? OR album LIKE ? OR genre LIKE ?
		ORDER BY artist, album, disc_num, track_num
		LIMIT 100`, pattern, pattern, pattern, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search (LIKE): %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanTracks(rows)
}

func buildPathList(n int) string {
	if n == 0 {
		return "''"
	}
	placeholders := "?"
	for i := 1; i < n; i++ {
		placeholders += ",?"
	}
	return placeholders
}

func mapToSlice(m map[string]bool) []any {
	s := make([]any, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	return s
}

func scanTracks(rows *sql.Rows) ([]models.Track, error) {
	var tracks []models.Track
	for rows.Next() {
		var t models.Track
		var hasCover int
		err := rows.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album,
			&t.AlbumArtist, &t.Year, &t.Genre, &t.TrackNum, &t.DiscNum,
			&t.Duration, &hasCover, &t.FileModTime)
		if err != nil {
			return nil, err
		}
		t.HasCoverArt = hasCover != 0
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}
