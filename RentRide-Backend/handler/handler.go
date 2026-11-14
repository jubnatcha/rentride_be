package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

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
// UpdateBookingStatus ใช้สำหรับอัปเดตสถานะการจอง
func (h *Handler) UpdateBookingStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขการจองต้องเป็นตัวเลข")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง")
		return
	}

	// สถานะที่อนุญาต (ต้องตรงกับที่ Frontend ส่ง และที่ DB รับ)
	allowed := map[string]bool{
		"pending":   true,
		"confirmed": true,
		"active":    true,
		"completed": true,
		"cancelled": true,
	}
	if !allowed[req.Status] {
		writeError(w, http.StatusBadRequest, "INVALID_STATUS", "สถานะการจองไม่ถูกต้อง: "+req.Status)
		return
	}

	err := h.Store.UpdateBookingStatus(r.Context(), id, req.Status)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "BOOKING_NOT_FOUND", "ไม่พบการจองที่เปลี่ยนสถานะได้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "อัปเดตสถานะสำเร็จ"})
}

var validTypes = map[string]string{
	"sedan": "Sedan", "suv": "SUV", "scooter": "Scooter", "bigbike": "BigBike",
}

func validateCar(in *store.CarInput) string {
	if in.Brand == "" || in.Model == "" {
		return "กรุณากรอกยี่ห้อและรุ่น"
	}
	t, ok := validTypes[strings.ToLower(strings.TrimSpace(in.Type))]
	if !ok {
		return "ประเภทรถไม่ถูกต้อง"
	}

	in.Type = t // แปลงเป็น Sedan / SUV / Scooter / BigBike
	if in.Type == "" {
	in.Type = in.CarType
}
	if in.PricePerDay <= 0 {
		return "ราคาต่อวันต้องมากกว่า 0"
	}
	return ""
}

func (h *Handler) CreateCar(w http.ResponseWriter, r *http.Request) {
	var in store.CarInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง")
		return
	}
	if msg := validateCar(&in); msg != "" {
		writeError(w, http.StatusBadRequest, "INVALID_CAR", msg)
		return
	}
	car, err := h.Store.CreateCar(r.Context(), in)
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, car)
}

func (h *Handler) UpdateCar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขรถต้องเป็นตัวเลข")
		return
	}
	var in store.CarInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "รูปแบบข้อมูลไม่ถูกต้อง")
		return
	}
	if msg := validateCar(&in); msg != "" {
		writeError(w, http.StatusBadRequest, "INVALID_CAR", msg)
		return
	}
	car, err := h.Store.UpdateCar(r.Context(), id, in)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "CAR_NOT_FOUND", "ไม่พบรถคันนี้")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, car)
}

func (h *Handler) DeleteCar(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "BAD_ID", "เลขรถต้องเป็นตัวเลข")
		return
	}
	err := h.Store.DeleteCar(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "CAR_NOT_FOUND", "ไม่พบรถคันนี้")
		return
	}
	if errors.Is(err, store.ErrCarInUse) {
		writeError(w, http.StatusConflict, "CAR_IN_USE", "ลบไม่ได้ เพราะรถคันนี้มีประวัติการจอง")
		return
	}
	if err != nil {
		internal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "ลบรถสำเร็จ"})
}