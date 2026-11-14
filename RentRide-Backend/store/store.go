package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)


var ErrNotFound = errors.New("not found")

type Store struct {
	Pool *pgxpool.Pool
}
// ErrCarInUse ใช้เมื่อลบรถไม่ได้เพราะมีการจองอ้างอิงอยู่
var ErrCarInUse = errors.New("car has bookings")

// CarInput คือข้อมูลที่ Admin ส่งมาตอนเพิ่ม/แก้ไขรถ
type CarInput struct {
	Brand       string  `json:"brand"`
	Model       string  `json:"model"`
	Type        string  `json:"type"`
	CarType      string  `json:"car_type"`
	Seats       *int    `json:"seats"`
	PricePerDay float64 `json:"price_per_day"`
	ImageURL    *string `json:"image_url"`
	Status      string  `json:"status"`
}

func (s *Store) CreateCar(ctx context.Context, in CarInput) (Car, error) {
	if in.Status == "" {
		in.Status = "available"
	}
	var c Car
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO cars (brand, model, type, seats, price_per_day, image_url, status)
         VALUES ($1, $2, $3, $4, $5, $6, $7)
         RETURNING id, brand, model, type, seats, price_per_day::float8, image_url, status`,
		in.Brand, in.Model, in.Type, in.Seats, in.PricePerDay, in.ImageURL, in.Status).
		Scan(&c.ID, &c.Brand, &c.Model, &c.Type, &c.Seats, &c.PricePerDay, &c.ImageURL, &c.Status)
	return c, err
}

func (s *Store) UpdateCar(ctx context.Context, id int64, in CarInput) (Car, error) {
	if in.Status == "" {
		in.Status = "available"
	}
	var c Car
	err := s.Pool.QueryRow(ctx,
		`UPDATE cars
         SET brand=$2, model=$3, type=$4, seats=$5, price_per_day=$6, image_url=$7, status=$8
         WHERE id=$1
         RETURNING id, brand, model, type, seats, price_per_day::float8, image_url, status`,
		id, in.Brand, in.Model, in.Type, in.Seats, in.PricePerDay, in.ImageURL, in.Status).
		Scan(&c.ID, &c.Brand, &c.Model, &c.Type, &c.Seats, &c.PricePerDay, &c.ImageURL, &c.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Car{}, ErrNotFound
	}
	return c, err
}

func (s *Store) DeleteCar(ctx context.Context, id int64) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM cars WHERE id = $1`, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign key violation
		return ErrCarInUse
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}