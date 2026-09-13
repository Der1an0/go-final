package handlers

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Der1an0/tasks"
)

// Static возвращает http.Handler для раздачи файлов из указанной директории
func Static(webDir string) http.Handler {
	return http.FileServer(http.Dir(webDir))
}

// GetPort возвращает порт из переменной окружения TODO_PORT,
// если переменная не задана или некорректна – возвращает 7540
func GetPort() int {
	portStr := os.Getenv("TODO_PORT")
	if portStr == "" {
		return 7540
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		log.Printf("Некорректное значение TODO_PORT: %s, используем порт по умолчанию 7540", portStr)
		return 7540
	}
	return port
}
func NextDateHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем, что метод именно GET
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем параметры из Query String
	nowStr := r.URL.Query().Get("now")
	dateStr := r.URL.Query().Get("date")
	repeat := r.URL.Query().Get("repeat")

	// Парсим дату "now" из формата "20060102"
	now, err := time.Parse("20060102", nowStr)
	if err != nil {
		http.Error(w, "Invalid 'now' date format", http.StatusBadRequest)
		return
	}

	// Вызываем ранее написанную логику
	nextDate, err := tasks.NextDate(now, dateStr, repeat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Тесты ждут чистую строку с датой в качестве ответа
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(nextDate))
}
