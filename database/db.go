// Package database
package database

import (
	"context"
	"log/slog"
	"os"
	generated "spidey/database/sqlc"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBService struct {
	pool    *pgxpool.Pool
	queries *generated.Queries
}

func NewDBService(ctx context.Context) (*DBService, error) {
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	slog.Info("Successfully connected to the database")

	return &DBService{
		pool:    pool,
		queries: generated.New(pool),
	}, nil
}

func (s *DBService) Close() {
	s.pool.Close()
}

func (s *DBService) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

func (s *DBService) QueriesWithTx(tx pgx.Tx) *generated.Queries {
	return s.queries.WithTx(tx)
}

func (s *DBService) ExecTx(ctx context.Context, fn func(*generated.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	err = fn(qtx)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
