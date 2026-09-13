package db

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite" // Импортируем драйвер для регистрации в sql.Open
)

// Глобальная переменная для хранения идентификатора открытой базы данных
var DB *sql.DB

// Структура задачи для JSON
type Tasks struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

// SQL-схема для создания таблицы и индекса
const schema = `
CREATE TABLE scheduler (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date CHAR(8) NOT NULL DEFAULT "",
    title VARCHAR(255) NOT NULL DEFAULT "",
    comment TEXT NOT NULL DEFAULT "",
    repeat VARCHAR(128) NOT NULL DEFAULT ""
);
CREATE INDEX idx_scheduler_date ON scheduler(date);
`

// Init проверяет наличие файла БД, открывает соединение и при необходимости создает таблицы
func Init(dbFile string) error {
	// Проверяем существование файла базы данных
	_, err := os.Stat(dbFile)
	var install bool
	if err != nil {
		if os.IsNotExist(err) {
			install = true
		} else {
			return err // Возвращаем ошибку, если проблема не в отсутствии файла
		}
	}

	// Открываем базу данных, используя драйвер "sqlite"
	var dbErr error
	DB, dbErr = sql.Open("sqlite", dbFile)
	if dbErr != nil {
		return dbErr
	}

	// Проверяем соединение с базой данных
	if err := DB.Ping(); err != nil {
		return err
	}

	// Если файла не существовало, выполняем sql-запрос для создания таблицы и индекса
	if install {
		log.Println("Файл базы данных не найден. Создаём таблицу и индекс...")
		_, err = DB.Exec(schema)
		if err != nil {
			DB.Close() // Закрываем соединение в случае ошибки создания схемы
			return err
		}
		log.Println("База данных успешно инициализирована.")
	}

	return nil
}

// Запись в таблицу
func AddTask(task *Tasks) (int64, error) {
	var id int64
	query := `INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)`
	res, err := DB.Exec(query, task.Date, task.Title, task.Comment, task.Repeat)
	if err == nil {
		id, err = res.LastInsertId()
	}
	return id, err
}
