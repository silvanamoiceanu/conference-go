package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
	pgxvector "github.com/pgvector/pgvector-go/pgx"

	"github.com/giorgio/conference-go/pkg/types"
)

type DB struct {
	pool *pgxpool.Pool
}

type Entry struct {
	DescHash  string
	Person    *types.Person
	Embedding []float32
}

func Open(ctx context.Context, connStr string) (*DB, error) {
	// Use a plain connection first to ensure the vector extension exists
	// before the pool tries to register the vector type in AfterConnect.
	bootstrap, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if _, err := bootstrap.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS vector`); err != nil {
		bootstrap.Close(ctx)
		return nil, fmt.Errorf("create vector extension: %w", err)
	}
	bootstrap.Close(ctx)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	config.MaxConns = 5
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvector.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS profiles (
		id SERIAL PRIMARY KEY,
		desc_hash TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		title TEXT,
		company TEXT,
		interests TEXT,
		skills TEXT,
		goals TEXT,
		description TEXT,
		embedding vector(3072)
	)`); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (d *DB) Close() {
	d.pool.Close()
}

func (d *DB) GetAll(ctx context.Context) ([]*Entry, error) {
	rows, err := d.pool.Query(ctx, `SELECT desc_hash, name, title, company, interests, skills, goals, description FROM profiles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*Entry
	for rows.Next() {
		e := &Entry{Person: &types.Person{}}
		var interests, skills, goals string
		if err := rows.Scan(&e.DescHash, &e.Person.Name, &e.Person.Title, &e.Person.Company,
			&interests, &skills, &goals, &e.Person.Description); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(interests), &e.Person.Interests)
		json.Unmarshal([]byte(skills), &e.Person.Skills)
		json.Unmarshal([]byte(goals), &e.Person.Goals)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (d *DB) Has(ctx context.Context, hash string) bool {
	var n int
	d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE desc_hash = $1`, hash).Scan(&n)
	return n > 0
}

func (d *DB) Save(ctx context.Context, e *Entry) error {
	interests, _ := json.Marshal(e.Person.Interests)
	skills, _ := json.Marshal(e.Person.Skills)
	goals, _ := json.Marshal(e.Person.Goals)
	vec := pgvector.NewVector(e.Embedding)
	_, err := d.pool.Exec(ctx,
		`INSERT INTO profiles (desc_hash, name, title, company, interests, skills, goals, description, embedding)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (desc_hash) DO UPDATE SET
		 name=EXCLUDED.name, title=EXCLUDED.title, company=EXCLUDED.company,
		 interests=EXCLUDED.interests, skills=EXCLUDED.skills, goals=EXCLUDED.goals,
		 description=EXCLUDED.description, embedding=EXCLUDED.embedding`,
		e.DescHash, e.Person.Name, e.Person.Title, e.Person.Company,
		string(interests), string(skills), string(goals), e.Person.Description, vec,
	)
	return err
}

func (d *DB) SearchByEmbedding(ctx context.Context, emb []float32, topK int) ([]*types.Person, []float32, error) {
	vec := pgvector.NewVector(emb)
	rows, err := d.pool.Query(ctx,
		`SELECT name, title, company, interests, skills, goals, description,
		        1 - (embedding <=> $1) AS similarity
		 FROM profiles
		 ORDER BY embedding <=> $1
		 LIMIT $2`,
		vec, topK,
	)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var persons []*types.Person
	var sims []float32
	for rows.Next() {
		p := &types.Person{}
		var interests, skills, goals string
		var sim float32
		if err := rows.Scan(&p.Name, &p.Title, &p.Company, &interests, &skills, &goals, &p.Description, &sim); err != nil {
			return nil, nil, err
		}
		json.Unmarshal([]byte(interests), &p.Interests)
		json.Unmarshal([]byte(skills), &p.Skills)
		json.Unmarshal([]byte(goals), &p.Goals)
		persons = append(persons, p)
		sims = append(sims, sim)
	}
	return persons, sims, rows.Err()
}
