package database

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

	_ "modernc.org/sqlite"

	"github.com/giorgio/conference-go/pkg/types"
)

type DB struct {
	db *sql.DB
}

type Entry struct {
	DescHash  string
	Person    *types.Person
	Embedding []float32
}

func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS profiles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		desc_hash TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		title TEXT,
		company TEXT,
		interests TEXT,
		skills TEXT,
		goals TEXT,
		description TEXT,
		embedding BLOB
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) GetAll() ([]*Entry, error) {
	rows, err := d.db.Query(`SELECT desc_hash, name, title, company, interests, skills, goals, description, embedding FROM profiles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		e := &Entry{Person: &types.Person{}}
		var interests, skills, goals string
		var embBlob []byte
		if err := rows.Scan(&e.DescHash, &e.Person.Name, &e.Person.Title, &e.Person.Company,
			&interests, &skills, &goals, &e.Person.Description, &embBlob); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(interests), &e.Person.Interests)
		json.Unmarshal([]byte(skills), &e.Person.Skills)
		json.Unmarshal([]byte(goals), &e.Person.Goals)
		e.Embedding = blobToFloat32(embBlob)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) Has(hash string) bool {
	var n int
	d.db.QueryRow(`SELECT COUNT(*) FROM profiles WHERE desc_hash = ?`, hash).Scan(&n)
	return n > 0
}

func (d *DB) Save(e *Entry) error {
	interests, _ := json.Marshal(e.Person.Interests)
	skills, _ := json.Marshal(e.Person.Skills)
	goals, _ := json.Marshal(e.Person.Goals)
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO profiles (desc_hash, name, title, company, interests, skills, goals, description, embedding) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.DescHash, e.Person.Name, e.Person.Title, e.Person.Company,
		string(interests), string(skills), string(goals), e.Person.Description, float32ToBlob(e.Embedding),
	)
	return err
}

func float32ToBlob(f []float32) []byte {
	b := make([]byte, len(f)*4)
	for i, v := range f {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}

func blobToFloat32(b []byte) []float32 {
	f := make([]float32, len(b)/4)
	for i := range f {
		f[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return f
}
