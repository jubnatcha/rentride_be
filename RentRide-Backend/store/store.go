package store

import (
	"errors"
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	Pool *pgxpool.Pool
}
// UpdateBookingStatus อัปเดตสถานะของการจองในฐานข้อมูล
func (s *Store) UpdateBookingStatus(ctx context.Context, id int64, status string) error {
	// สมมติว่าตารางชื่อ bookings และใช้ $1, $2 (PostgreSQL) หรือ ?, ? (MySQL)
	query := `UPDATE bookings SET status = $1 WHERE id = $2`
	
	// s.Pool คือ connection ฐานข้อมูลของคุณที่ถูกประกาศไว้ใน type Store
	_, err := s.Pool.Exec(ctx, query, status, id)
	if err != nil {
		return err
	}
	
	return nil
}