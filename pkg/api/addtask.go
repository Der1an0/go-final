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
// Главный распределитель запросов /api/task по HTTP-методам
func TaskHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		addTaskHandler(w, r)
	case http.MethodGet:
		getTaskHandler(w, r) // Добавили обработку GET
	case http.MethodPut:
		updateTaskHandler(w, r) // Добавили обработку PUT
	case http.MethodDelete:
		deleteTaskHandler(w, r) // Добавили обработку DELETE
	default:
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

// getTaskHandler обрабатывает GET /api/task?id=...
func getTaskHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "Не указан идентификатор"})
		return
	}

	// Запрашиваем задачу из базы данных
	task, err := db.GetTask(id)
	if err != nil {
		// Тесты Практикума ожидают понятную ошибку, если задача отсутствует в базе
		writeJson(w, http.StatusNotFound, map[string]string{"error": "Задача не найдена"})
		return
	}

	// Возвращаем JSON-объект задачи
	writeJson(w, http.StatusOK, task)
}

// updateTaskHandler обрабатывает PUT /api/task
func updateTaskHandler(w http.ResponseWriter, r *http.Request) {
	var task db.Task

	// Десериализуем входящий JSON изменения задачи
	err := json.NewDecoder(r.Body).Decode(&task)
	if err != nil {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "Ошибка десериализации JSON"})
		return
	}

	// Валидация: ID обязателен для изменения
	if task.ID == "" {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "Не указан идентификатор"})
		return
	}

	// Валидация: заголовок обязателен
	if task.Title == "" {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "Не указан заголовок задачи"})
		return
	}

	// Проверяем и корректируем дату задачи (используем ту же функцию, что и для POST)
	if err := checkDate(&task); err != nil {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Отправляем изменения в базу данных
	err = db.UpdateTask(&task)
	if err != nil {
		writeJson(w, http.StatusNotFound, map[string]string{"error": "Задача не найдена"})
		return
	}

	// В случае успешного изменения возвращаем пустой JSON объект {}
	writeJson(w, http.StatusOK, map[string]string{})
}

// deleteTaskHandler обрабатывает DELETE /api/task?id=...
func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJson(w, http.StatusBadRequest, map[string]string{"error": "Не указан идентификатор"})
		return
	}

	err := db.DeleteTask(id)
	if err != nil {
		writeJson(w, http.StatusNotFound, map[string]string{"error": "Задача не найдена"})
		return
	}

	writeJson(w, http.StatusOK, map[string]string{})
}

// TaskDoneHandler обрабатывает POST /api/task/done?id=...
func TaskDoneHandler(w http.ResponseWriter, r *http.Request) {
	// Разрешаем только POST запросы
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Не указан идентификатор"})
		return
	}

	// 1. Получаем параметры задачи по её id
	task, err := db.GetTask(id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Задача не найдена"})
		return
	}

	// 2. Если правило для повторения отсутствует, удаляем задачу
	if task.Repeat == "" {
		err = db.DeleteTask(id)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=UTF-8")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
		return
	}

	// 3. Если задача периодическая, рассчитываем следующую дату
	now := time.Now()
	nextDate, err := db.NextDate(now, task.Date, task.Repeat)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Обновляем дату у задачи в базе данных
	err = db.UpdateTaskDate(id, nextDate)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{})
}
