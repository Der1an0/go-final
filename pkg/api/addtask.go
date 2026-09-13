package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Der1an0/pkg/db"
)

// Вспомогательная функция для отправки JSON-ответов
func writeJson(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Главный распределитель запросов /api/task по HTTP-методам
func TaskHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		addTaskHandler(w, r)
	default:
		// Возвращаем ошибку строго в JSON, а не через http.Error (который шлет обычный текст)
		writeJson(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
	}
}

// Вспомогательная функция проверки и нормализации дат
func checkDate(task *db.Tasks) error {
	now := time.Now()
	todayStr := now.Format("20060102")

	// 1. Если дата пустая — берем сегодня
	if task.Date == "" {
		task.Date = todayStr
		return nil
	}

	// 2. Проверяем валидность формата 20060102
	t, err := time.Parse("20060102", task.Date)
	if err != nil {
		return errors.New("некорректный формат даты")
	}

	// Сбрасываем время у текущей даты для точного сравнения дней
	nowZero := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 3. Если дата в прошлом
	if t.Before(nowZero) {
		if len(task.Repeat) == 0 {
			// Если правила повторения нет — берем сегодняшнее число
			task.Date = todayStr
		} else {
			// Иначе вычисляем следующую дату через планировщик
			next, err := db.NextDate(now, task.Date, task.Repeat)
			if err != nil {
				return err
			}
			task.Date = next
		}
	}

	return nil
}

// Логика обработки POST-запроса
func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task db.Tasks

	// Читаем тело JSON
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "Ошибка десериализации JSON"})
		return
	}

	// Проверяем обязательное поле Title
	if task.Title == "" {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "Не указан заголовок задачи"})
		return
	}

	// Проверяем и корректируем дату задачи
	if err := checkDate(&task); err != nil {
		// Любая ошибка валидации даты должна уходить как JSON {"error": "..."}
		writeJson(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Вызываем функцию сохранения в БД из пакета db
	id, err := db.AddTask(&task)
	if err != nil {
		writeJson(w, http.StatusInternalServerError, map[string]string{"error": "Ошибка записи в базу данных"})
		return
	}

	// Возвращаем успешный ID в JSON-формате, как ждут тесты
	writeJson(w, http.StatusOK, map[string]int64{"id": id})
}
