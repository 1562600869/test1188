package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp APIResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func successResponse(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: data})
}

func errorResponse(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, APIResponse{Success: false, Message: msg})
}

func readBody(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, v)
}

func handleCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func parseIntParam(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

func handleRooms(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/rooms")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" && r.Method == "GET":
		rooms, err := getAllRooms()
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		successResponse(w, rooms)

	case path == "" && r.Method == "POST":
		var room Room
		if err := readBody(r, &room); err != nil {
			errorResponse(w, http.StatusBadRequest, "请求体格式错误: "+err.Error())
			return
		}
		if room.RoomNumber == "" || room.RoomType == "" || room.PricePerNight <= 0 || room.MaxGuests <= 0 {
			errorResponse(w, http.StatusBadRequest, "房间号、类型、价格、最大入住人数不能为空或小于等于0")
			return
		}
		if room.Status == "" {
			room.Status = "空闲"
		}
		if err := createRoom(&room); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		successResponse(w, room)

	case path != "" && r.Method == "GET":
		id, err := parseIntParam(path)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的房间ID")
			return
		}
		room, err := getRoomByID(id)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if room == nil {
			errorResponse(w, http.StatusNotFound, "房间不存在")
			return
		}
		successResponse(w, room)

	case path != "" && r.Method == "PUT":
		id, err := parseIntParam(path)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的房间ID")
			return
		}
		var room Room
		if err := readBody(r, &room); err != nil {
			errorResponse(w, http.StatusBadRequest, "请求体格式错误: "+err.Error())
			return
		}
		room.ID = id
		if err := updateRoom(&room); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		successResponse(w, room)

	case path != "" && r.Method == "DELETE":
		id, err := parseIntParam(path)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的房间ID")
			return
		}
		if err := deleteRoom(id); err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		successResponse(w, map[string]string{"message": "删除成功"})

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func handleBookings(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/bookings")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" && r.Method == "GET":
		bookings, err := getAllBookings()
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		successResponse(w, bookings)

	case path == "" && r.Method == "POST":
		var booking Booking
		if err := readBody(r, &booking); err != nil {
			errorResponse(w, http.StatusBadRequest, "请求体格式错误: "+err.Error())
			return
		}
		if booking.GuestName == "" || booking.GuestPhone == "" || booking.RoomID <= 0 ||
			booking.CheckInDate == "" || booking.CheckOutDate == "" || booking.GuestCount <= 0 {
			errorResponse(w, http.StatusBadRequest, "客人信息、房间、日期、人数不能为空")
			return
		}
		if err := createBooking(&booking); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		successResponse(w, booking)

	case strings.HasSuffix(path, "/checkin") && r.Method == "POST":
		idStr := strings.TrimSuffix(path, "/checkin")
		id, err := parseIntParam(idStr)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的预订ID")
			return
		}
		if err := confirmCheckIn(id); err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		booking, _ := getBookingByID(id)
		successResponse(w, booking)

	case strings.HasSuffix(path, "/checkout") && r.Method == "POST":
		idStr := strings.TrimSuffix(path, "/checkout")
		id, err := parseIntParam(idStr)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的预订ID")
			return
		}
		var req struct {
			ActualCheckOut string `json:"actual_check_out"`
		}
		readBody(r, &req)
		totalPrice, err := checkOutSettle(id, req.ActualCheckOut)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		booking, _ := getBookingByID(id)
		successResponse(w, map[string]interface{}{
			"booking":     booking,
			"total_price": totalPrice,
		})

	case path != "" && r.Method == "GET":
		id, err := parseIntParam(path)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "无效的预订ID")
			return
		}
		booking, err := getBookingByID(id)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		if booking == nil {
			errorResponse(w, http.StatusNotFound, "预订不存在")
			return
		}
		successResponse(w, booking)

	default:
		errorResponse(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func handleTodayCheckIn(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}
	if r.Method != "GET" {
		errorResponse(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	bookings, err := getTodayCheckIn()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	successResponse(w, bookings)
}

func handleTodayCheckOut(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}
	if r.Method != "GET" {
		errorResponse(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	bookings, err := getTodayCheckOut()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	successResponse(w, bookings)
}

func handleMonthlyStats(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}
	if r.Method != "GET" {
		errorResponse(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	stats, err := getMonthlyStats()
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	successResponse(w, stats)
}

func handleAvailability(w http.ResponseWriter, r *http.Request) {
	if handleCORS(w, r) {
		return
	}
	if r.Method != "GET" {
		errorResponse(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	roomIDStr := r.URL.Query().Get("room_id")
	checkIn := r.URL.Query().Get("check_in")
	checkOut := r.URL.Query().Get("check_out")

	roomID, err := parseIntParam(roomIDStr)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, "无效的房间ID")
		return
	}
	if checkIn == "" || checkOut == "" {
		errorResponse(w, http.StatusBadRequest, "入住日期和离店日期不能为空")
		return
	}

	available, err := checkRoomAvailability(roomID, checkIn, checkOut, 0)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	successResponse(w, map[string]bool{"available": available})
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, indexHTML)
}
