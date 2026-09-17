package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Der1an0/handlers"
	"github.com/Der1an0/pkg/api"
	"github.com/Der1an0/pkg/db"
)

func main() {
	// Определяем имя файла базы данных. Если задана переменная окружения, берем её.
	dbFile := os.Getenv("TODO_DBFILE")
	if dbFile == "" {
		dbFile = "scheduler.db"
	}

	// Инициализируем базу данных
	err := db.Init(dbFile)
	if err != nil {
		log.Fatalf("Ошибка инициализации базы данных: %v", err)
	}
	defer db.DB.Close() // Закрываем БД при завершении работы программы
	port := handlers.GetPort()
	http.HandleFunc("/api/nextdate", handlers.NextDateHandler)
	http.HandleFunc("/api/task", api.TaskHandler)
	http.HandleFunc("/api/tasks", api.TasksHandler)
	webDir := "./web"

	http.Handle("/", handlers.Static(webDir))

	log.Printf("Сервер запущен на http://localhost:%d", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
