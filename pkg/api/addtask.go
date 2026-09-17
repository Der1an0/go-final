package api

import (
	"encoding/json"
	"errors"
	"log"
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

type TasksResp struct {
	Tasks []*db.Task `json:"tasks"`
}

type ErrorResp struct {
	Error string `json:"error"`
}

func TasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResp{Error: "Method not allowed"})
		return
	}

	// Вытаскиваем параметр search из URL
	searchParam := r.URL.Query().Get("search")
	log.Printf("TasksHandler: search=%q", searchParam)

	// Передаем ЕГО вторым аргументом!
	tasks, err := db.GetTasks(50, searchParam)

	if err != nil {
		log.Printf("GetTasks error: %v", err)
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResp{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TasksResp{
		Tasks: tasks,
	})
	log.Printf("GetTasks returned %d tasks", len(tasks))
}

// Вспомогательная функция проверки и нормализации дат
func checkDate(task *db.Task) error {
	now := time.Now()
	// Сбрасываем время у текущей даты, чтобы сравнивать только чистые дни (00:00:00)
	nowZero := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayStr := nowZero.Format("20060102")

	// 1. Если дата пустая — берем сегодняшний день
	if task.Date == "" {
		task.Date = todayStr
		return nil
	}

	// 2. Проверяем валидность формата 20060102
	t, err := time.ParseInLocation("20060102", task.Date, time.Local)
	if err != nil {
		return errors.New("некорректный формат даты")
	}

	// 3. Если дата в прошлом (строго до сегодняшнего дня 00:00:00)
	if t.Before(nowZero) {
		if len(task.Repeat) == 0 {
			// Если правила повторения нет — ставим сегодняшний день
			task.Date = todayStr
		} else {
			// Если правило есть — вычисляем СЛЕДУЮЩУЮ дату относительно NOW
			next, err := db.NextDate(nowZero, task.Date, task.Repeat)
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
	var task db.Task

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
