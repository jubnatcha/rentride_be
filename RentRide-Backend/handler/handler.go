package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"rentride/auth"
	"rentride/store"
)

type Handler struct {
	Store   *store.Store
	Tickets *auth.Issuer
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

// internal จดรายละเอียดไว้ใน log ฝั่งเรา แต่บอกผู้ใช้แค่ว่าระบบขัดข้อง
func internal(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// Health ใช้เช็คว่าเซิร์ฟเวอร์และฐานข้อมูลยังอยู่ (Render ใช้ path นี้เป็น Health Check ได้)
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.Pool.Ping(r.Context()); err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ListCars(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := h.Store.ListCars(r.Context(), q.Get("q"), q.Get("type"))
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetCar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขรถต้องเป็นตัวเลข")
		return
	}
	c, err := h.Store.GetCar(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "CAR_NOT_FOUND", "ไม่พบรถคันนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}
// UpdateBookingStatus ใช้สำหรับอัปเดตสถานะการจอง
func (h *Handler) UpdateBookingStatus(w http.ResponseWriter, r *http.Request) {
	// 1. ดึง ID ของการจองจาก URL
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขการจองต้องเป็นตัวเลข")
		return
	}

	// 2. รับค่าสถานะใหม่ที่ส่งมาทาง Request Body (JSON)
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง")
		return
	}

	// 3. เรียกใช้ Store เพื่ออัปเดตข้อมูลในฐานข้อมูล
	err := h.Store.UpdateBookingStatus(r.Context(), id, req.Status)
	if err != nil {
		internal(w, err)
		return
	}

	// 4. ส่งผลลัพธ์กลับไปว่าอัปเดตสำเร็จ
	writeJSON(w, http.StatusOK, map[string]string{"message": "อัปเดตสถานะสำเร็จ"})
}