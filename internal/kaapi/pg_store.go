package kaapi

import (
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenPostgresStore opens a PostgreSQL registry (Phase 2).
// dsn example: postgres://user:pass@localhost:5432/coe?sslmode=disable
func OpenPostgresStore(dsn string) (*SQLStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Hour)
	s := &SQLStore{db: db, dialect: "postgres"}
	if err := s.migratePG(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLStore) migratePG() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS meta (
  k TEXT PRIMARY KEY,
  v TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS devices (
  device_id TEXT PRIMARY KEY,
  identity_pk BYTEA NOT NULL,
  sign_pk BYTEA,
  org_id TEXT,
  label TEXT,
  enrollment_counter BIGINT NOT NULL,
  profile TEXT NOT NULL,
  revoked INTEGER NOT NULL DEFAULT 0,
  last_epoch_id BIGINT,
  last_serial BIGINT,
  issued_epoch BIGINT
);
CREATE TABLE IF NOT EXISTS vouchers (
  id TEXT PRIMARY KEY,
  code_hash TEXT NOT NULL UNIQUE,
  label TEXT,
  org_id TEXT,
  profile TEXT,
  max_uses INTEGER NOT NULL,
  uses INTEGER NOT NULL DEFAULT 0,
  expires_at BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL,
  revoked INTEGER NOT NULL DEFAULT 0
);
INSERT INTO meta(k,v) VALUES('serial','0') ON CONFLICT (k) DO NOTHING;
`)
	return err
}
