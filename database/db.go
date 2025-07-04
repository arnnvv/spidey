// Package database
package database

import (
	"context"
	"log/slog"
	"os"
	"spidey/database/generated"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBService struct {
	Pool    *pgxpool.Pool
	Queries *generated.Queries
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
		Pool:    pool,
		Queries: generated.New(pool),
	}, nil
}

func (s *DBService) Close() {
	s.Pool.Close()
}

func (s *DBService) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return s.Pool.Begin(ctx)
}

func (s *DBService) QueriesWithTx(tx pgx.Tx) *generated.Queries {
	return s.Queries.WithTx(tx)
}

func (s *DBService) ExecTx(ctx context.Context, fn func(*generated.Queries) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.Queries.WithTx(tx)

	err = fn(qtx)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
