package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
)

var indexHTML string

func loadIndexHTML() error {
	data, err := os.ReadFile("index.html")
	if err != nil {
		return err
	}
	indexHTML = string(data)
	return nil
}

func main() {
	port := flag.String("port", "8293", "服务器监听端口")
	flag.Parse()

	if err := initDB(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()
	log.Println("数据库初始化完成")

	if err := loadIndexHTML(); err != nil {
		log.Fatalf("加载首页失败: %v", err)
	}
	log.Println("首页加载完成")

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || strings.HasSuffix(r.URL.Path, ".html") {
			handleIndex(w, r)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/api/rooms", handleRooms)
	mux.HandleFunc("/api/rooms/", handleRooms)
	mux.HandleFunc("/api/bookings", handleBookings)
	mux.HandleFunc("/api/bookings/", handleBookings)
	mux.HandleFunc("/api/today-checkin", handleTodayCheckIn)
	mux.HandleFunc("/api/today-checkout", handleTodayCheckOut)
	mux.HandleFunc("/api/monthly-stats", handleMonthlyStats)
	mux.HandleFunc("/api/availability", handleAvailability)

	addr := ":" + *port
	log.Printf("服务器启动在 http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
