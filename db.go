package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Room struct {
	ID           int    `json:"id"`
	RoomNumber   string `json:"room_number"`
	RoomType     string `json:"room_type"`
	PricePerNight int   `json:"price_per_night"`
	MaxGuests    int    `json:"max_guests"`
	Status       string `json:"status"`
}

type Booking struct {
	ID           int    `json:"id"`
	GuestName    string `json:"guest_name"`
	GuestPhone   string `json:"guest_phone"`
	RoomID       int    `json:"room_id"`
	RoomNumber   string `json:"room_number"`
	RoomType     string `json:"room_type"`
	CheckInDate  string `json:"check_in_date"`
	CheckOutDate string `json:"check_out_date"`
	GuestCount   int    `json:"guest_count"`
	Status       string `json:"status"`
	TotalPrice   int    `json:"total_price"`
	CreatedAt    string `json:"created_at"`
}

type MonthlyStats struct {
	RoomType     string `json:"room_type"`
	StayCount    int    `json:"stay_count"`
	TotalRevenue int    `json:"total_revenue"`
}

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./hotel.db")
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS rooms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			room_number TEXT NOT NULL UNIQUE,
			room_type TEXT NOT NULL,
			price_per_night INTEGER NOT NULL,
			max_guests INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT '空闲'
		);

		CREATE TABLE IF NOT EXISTS bookings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guest_name TEXT NOT NULL,
			guest_phone TEXT NOT NULL,
			room_id INTEGER NOT NULL,
			check_in_date TEXT NOT NULL,
			check_out_date TEXT NOT NULL,
			guest_count INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT '已预订',
			total_price INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			FOREIGN KEY (room_id) REFERENCES rooms(id)
		);
	`)
	if err != nil {
		return err
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM rooms").Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		seedRooms := []Room{
			{RoomNumber: "101", RoomType: "标准间", PricePerNight: 29800, MaxGuests: 2, Status: "空闲"},
			{RoomNumber: "102", RoomType: "标准间", PricePerNight: 29800, MaxGuests: 2, Status: "空闲"},
			{RoomNumber: "103", RoomType: "大床房", PricePerNight: 39800, MaxGuests: 2, Status: "空闲"},
			{RoomNumber: "104", RoomType: "大床房", PricePerNight: 39800, MaxGuests: 2, Status: "空闲"},
			{RoomNumber: "201", RoomType: "家庭房", PricePerNight: 59800, MaxGuests: 4, Status: "空闲"},
			{RoomNumber: "202", RoomType: "家庭房", PricePerNight: 59800, MaxGuests: 4, Status: "空闲"},
			{RoomNumber: "301", RoomType: "套房", PricePerNight: 99800, MaxGuests: 3, Status: "空闲"},
			{RoomNumber: "302", RoomType: "套房", PricePerNight: 99800, MaxGuests: 3, Status: "维修"},
		}
		for _, r := range seedRooms {
			_, err = db.Exec(
				"INSERT INTO rooms (room_number, room_type, price_per_night, max_guests, status) VALUES (?, ?, ?, ?, ?)",
				r.RoomNumber, r.RoomType, r.PricePerNight, r.MaxGuests, r.Status,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func getAllRooms() ([]Room, error) {
	rows, err := db.Query("SELECT id, room_number, room_type, price_per_night, max_guests, status FROM rooms ORDER BY room_number")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []Room
	for rows.Next() {
		var r Room
		err = rows.Scan(&r.ID, &r.RoomNumber, &r.RoomType, &r.PricePerNight, &r.MaxGuests, &r.Status)
		if err != nil {
			return nil, err
		}
		rooms = append(rooms, r)
	}
	return rooms, nil
}

func getRoomByID(id int) (*Room, error) {
	var r Room
	err := db.QueryRow("SELECT id, room_number, room_type, price_per_night, max_guests, status FROM rooms WHERE id = ?", id).
		Scan(&r.ID, &r.RoomNumber, &r.RoomType, &r.PricePerNight, &r.MaxGuests, &r.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func createRoom(r *Room) error {
	result, err := db.Exec(
		"INSERT INTO rooms (room_number, room_type, price_per_night, max_guests, status) VALUES (?, ?, ?, ?, ?)",
		r.RoomNumber, r.RoomType, r.PricePerNight, r.MaxGuests, r.Status,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	r.ID = int(id)
	return nil
}

func updateRoom(r *Room) error {
	_, err := db.Exec(
		"UPDATE rooms SET room_number=?, room_type=?, price_per_night=?, max_guests=?, status=? WHERE id=?",
		r.RoomNumber, r.RoomType, r.PricePerNight, r.MaxGuests, r.Status, r.ID,
	)
	return err
}

func deleteRoom(id int) error {
	_, err := db.Exec("DELETE FROM rooms WHERE id=?", id)
	return err
}

func checkRoomAvailability(roomID int, checkIn, checkOut string, excludeBookingID int) (bool, error) {
	var count int
	query := `
		SELECT COUNT(*) FROM bookings
		WHERE room_id = ? AND status IN ('已预订', '已入住')
		AND check_in_date < ? AND check_out_date > ?
	`
	args := []interface{}{roomID, checkOut, checkIn}
	if excludeBookingID > 0 {
		query += " AND id != ?"
		args = append(args, excludeBookingID)
	}
	err := db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func createBooking(b *Booking) error {
	room, err := getRoomByID(b.RoomID)
	if err != nil {
		return err
	}
	if room == nil {
		return fmt.Errorf("房间不存在")
	}
	if room.Status == "维修" {
		return fmt.Errorf("该房间正在维修，无法预订")
	}
	if b.GuestCount > room.MaxGuests {
		return fmt.Errorf("人数超过房间最大入住人数 %d 人", room.MaxGuests)
	}

	available, err := checkRoomAvailability(b.RoomID, b.CheckInDate, b.CheckOutDate, 0)
	if err != nil {
		return err
	}
	if !available {
		return fmt.Errorf("该房间在指定时段已被预订")
	}

	checkInTime, err := time.Parse("2006-01-02", b.CheckInDate)
	if err != nil {
		return fmt.Errorf("入住日期格式错误")
	}
	checkOutTime, err := time.Parse("2006-01-02", b.CheckOutDate)
	if err != nil {
		return fmt.Errorf("离店日期格式错误")
	}
	days := int(checkOutTime.Sub(checkInTime).Hours() / 24)
	if days <= 0 {
		return fmt.Errorf("离店日期必须晚于入住日期")
	}
	b.TotalPrice = days * room.PricePerNight
	b.CreatedAt = time.Now().Format("2006-01-02 15:04:05")

	result, err := db.Exec(
		`INSERT INTO bookings (guest_name, guest_phone, room_id, check_in_date, check_out_date, guest_count, status, total_price, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.GuestName, b.GuestPhone, b.RoomID, b.CheckInDate, b.CheckOutDate, b.GuestCount, "已预订", b.TotalPrice, b.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = int(id)
	b.RoomNumber = room.RoomNumber
	b.RoomType = room.RoomType
	return nil
}

func scanBookingWithRoom(rows interface{ Scan(dest ...interface{}) error }) (*Booking, error) {
	var b Booking
	var roomNumber, roomType string
	err := rows.Scan(&b.ID, &b.GuestName, &b.GuestPhone, &b.RoomID, &b.CheckInDate, &b.CheckOutDate, &b.GuestCount, &b.Status, &b.TotalPrice, &b.CreatedAt, &roomNumber, &roomType)
	if err != nil {
		return nil, err
	}
	b.RoomNumber = roomNumber
	b.RoomType = roomType
	return &b, nil
}

func getAllBookings() ([]Booking, error) {
	rows, err := db.Query(`
		SELECT b.id, b.guest_name, b.guest_phone, b.room_id, b.check_in_date, b.check_out_date,
		       b.guest_count, b.status, b.total_price, b.created_at,
		       r.room_number, r.room_type
		FROM bookings b JOIN rooms r ON b.room_id = r.id
		ORDER BY b.id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []Booking
	for rows.Next() {
		b, err := scanBookingWithRoom(rows)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, *b)
	}
	return bookings, nil
}

func getBookingByID(id int) (*Booking, error) {
	row := db.QueryRow(`
		SELECT b.id, b.guest_name, b.guest_phone, b.room_id, b.check_in_date, b.check_out_date,
		       b.guest_count, b.status, b.total_price, b.created_at,
		       r.room_number, r.room_type
		FROM bookings b JOIN rooms r ON b.room_id = r.id
		WHERE b.id = ?
	`, id)
	b, err := scanBookingWithRoom(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return b, err
}

func confirmCheckIn(bookingID int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var roomID int
	var status string
	err = tx.QueryRow("SELECT room_id, status FROM bookings WHERE id = ?", bookingID).Scan(&roomID, &status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("预订不存在")
	}
	if err != nil {
		return err
	}
	if status != "已预订" {
		return fmt.Errorf("预订状态为 %s，无法办理入住", status)
	}

	_, err = tx.Exec("UPDATE bookings SET status = '已入住' WHERE id = ?", bookingID)
	if err != nil {
		return err
	}
	_, err = tx.Exec("UPDATE rooms SET status = '入住' WHERE id = ?", roomID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func checkOutSettle(bookingID int, actualCheckOut string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var roomID int
	var checkInDate string
	var originalPrice int
	var status string
	err = tx.QueryRow("SELECT room_id, check_in_date, total_price, status FROM bookings WHERE id = ?", bookingID).
		Scan(&roomID, &checkInDate, &originalPrice, &status)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("预订不存在")
	}
	if err != nil {
		return 0, err
	}
	if status != "已入住" {
		return 0, fmt.Errorf("预订状态为 %s，无法办理退房", status)
	}

	var pricePerNight int
	err = tx.QueryRow("SELECT price_per_night FROM rooms WHERE id = ?", roomID).Scan(&pricePerNight)
	if err != nil {
		return 0, err
	}

	var actualCheckOutDate string
	if actualCheckOut != "" {
		actualCheckOutDate = actualCheckOut
	} else {
		actualCheckOutDate = time.Now().Format("2006-01-02")
	}

	checkInTime, err := time.Parse("2006-01-02", checkInDate)
	if err != nil {
		return 0, fmt.Errorf("入住日期格式错误")
	}
	checkOutTime, err := time.Parse("2006-01-02", actualCheckOutDate)
	if err != nil {
		return 0, fmt.Errorf("实际离店日期格式错误")
	}
	days := int(checkOutTime.Sub(checkInTime).Hours() / 24)
	if days <= 0 {
		days = 1
	}
	totalPrice := days * pricePerNight

	_, err = tx.Exec("UPDATE bookings SET status = '已退房', check_out_date = ?, total_price = ? WHERE id = ?", actualCheckOutDate, totalPrice, bookingID)
	if err != nil {
		return 0, err
	}
	_, err = tx.Exec("UPDATE rooms SET status = '空闲' WHERE id = ?", roomID)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	return totalPrice, err
}

func getTodayCheckIn() ([]Booking, error) {
	today := time.Now().Format("2006-01-02")
	rows, err := db.Query(`
		SELECT b.id, b.guest_name, b.guest_phone, b.room_id, b.check_in_date, b.check_out_date,
		       b.guest_count, b.status, b.total_price, b.created_at,
		       r.room_number, r.room_type
		FROM bookings b JOIN rooms r ON b.room_id = r.id
		WHERE b.check_in_date = ? AND b.status IN ('已预订', '已入住')
		ORDER BY b.id DESC
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []Booking
	for rows.Next() {
		b, err := scanBookingWithRoom(rows)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, *b)
	}
	return bookings, nil
}

func getTodayCheckOut() ([]Booking, error) {
	today := time.Now().Format("2006-01-02")
	rows, err := db.Query(`
		SELECT b.id, b.guest_name, b.guest_phone, b.room_id, b.check_in_date, b.check_out_date,
		       b.guest_count, b.status, b.total_price, b.created_at,
		       r.room_number, r.room_type
		FROM bookings b JOIN rooms r ON b.room_id = r.id
		WHERE b.check_out_date = ? AND b.status IN ('已入住', '已退房')
		ORDER BY b.id DESC
	`, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []Booking
	for rows.Next() {
		b, err := scanBookingWithRoom(rows)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, *b)
	}
	return bookings, nil
}

func getMonthlyStats() ([]MonthlyStats, error) {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format("2006-01-02")
	endOfMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.Local).Format("2006-01-02")

	rows, err := db.Query(`
		SELECT r.room_type,
		       COUNT(b.id) as stay_count,
		       COALESCE(SUM(b.total_price), 0) as total_revenue
		FROM rooms r
		LEFT JOIN bookings b ON b.room_id = r.id
			AND b.status = '已退房'
			AND b.check_in_date >= ?
			AND b.check_in_date < ?
		GROUP BY r.room_type
		ORDER BY r.room_type
	`, startOfMonth, endOfMonth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []MonthlyStats
	for rows.Next() {
		var s MonthlyStats
		err = rows.Scan(&s.RoomType, &s.StayCount, &s.TotalRevenue)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}
