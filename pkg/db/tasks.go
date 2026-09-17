package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)

const dateLayout = "20060102"

// GetTasks возвращает список задач с возможностью фильтрации по строке поиска search
func GetTasks(limit int, search string) ([]*Task, error) {
	var cnt int
	DB.QueryRow("SELECT COUNT(*) FROM scheduler").Scan(&cnt)
	log.Printf("GetTasks: в БД всего %d задач, search=%q", cnt, search)
	search = strings.TrimSpace(search)

	var (
		rows *sql.Rows
		err  error
	)

	// Попытка распарсить как дату в формате ДД.ММ.ГГГГ
	if t, dateErr := time.ParseInLocation("02.01.2006", search, time.Local); search != "" && dateErr == nil {
		dateStr := t.Format("20060102")
		query := `SELECT id, date, title, comment, repeat FROM scheduler WHERE date = ? ORDER BY date LIMIT ?`
		rows, err = DB.Query(query, dateStr, limit)
	} else {
		query := `SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date LIMIT ?`
		rows, err = DB.Query(query, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	needle := strings.ToLower(search)
	tasks := make([]*Task, 0)
	for rows.Next() {
		var t Task
		var idInt int64
		if err := rows.Scan(&idInt, &t.Date, &t.Title, &t.Comment, &t.Repeat); err != nil {
			return nil, err
		}
		// Фильтр по подстроке в Go — работает и для кириллицы
		if search != "" && needle != "" {
			// Если это была дата — фильтр уже применён в SQL
			if _, dateErr := time.ParseInLocation("02.01.2006", search, time.Local); dateErr != nil {
				if !strings.Contains(strings.ToLower(t.Title), needle) &&
					!strings.Contains(strings.ToLower(t.Comment), needle) {
					continue
				}
			}
		}
		t.ID = strconv.FormatInt(idInt, 10)
		tasks = append(tasks, &t)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}

// NextDate вычисляет следующую дату задачи в соответствии с правилом repeat.
func NextDate(now time.Time, dstart string, repeat string) (string, error) {
	// 1. Валидация правила на пустую строку
	if strings.TrimSpace(repeat) == "" {
		return "", errors.New("repeat rule cannot be empty")
	}

	// 2. Парсинг стартовой даты
	startDate, err := time.Parse(dateLayout, dstart)
	if err != nil {
		return "", fmt.Errorf("failed to parse dstart: %w", err)
	}

	// Сбрасываем время у параметров, чтобы сравнивать только чистые даты
	nowZero := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Разбираем строку правила на токены
	tokens := strings.Fields(repeat)
	if len(tokens) == 0 {
		return "", errors.New("invalid repeat format")
	}

	ruleType := tokens[0]

	switch ruleType {
	case "d":
		// === Правило переноса по дням ===
		if len(tokens) < 2 {
			return "", errors.New("days interval not specified")
		}
		days, err := strconv.Atoi(tokens[1])
		if err != nil || days <= 0 {
			return "", errors.New("invalid days interval")
		}
		if days > 400 {
			return "", errors.New("days interval exceeds maximum limit of 400")
		}

		// Идём с шагом в days дней от startDate, пока не перешагнем через nowZero
		currentDate := startDate
		for {
			currentDate = currentDate.AddDate(0, 0, days)
			if currentDate.After(nowZero) {
				return currentDate.Format(dateLayout), nil
			}
		}

	case "y":
		// === Правило переноса по годам ===
		currentDate := startDate
		for {
			currentDate = currentDate.AddDate(1, 0, 0)
			if currentDate.After(nowZero) {
				return currentDate.Format(dateLayout), nil
			}
		}

	case "w":
		// === Правило дней недели (со звездочкой) ===
		if len(tokens) < 2 {
			return "", errors.New("weekdays not specified")
		}

		weekdayTokens := strings.Split(tokens[1], ",")
		weekdayMap := make(map[int]bool)
		for _, token := range weekdayTokens {
			day, err := strconv.Atoi(token)
			if err != nil || day < 1 || day > 7 {
				return "", errors.New("invalid weekday value")
			}
			weekdayMap[day] = true
		}

		// Перебираем дни посуточно, начиная со следующего дня после базовой точки отсчета
		// Базовая точка — это максимум между startDate и теперь (сдвинутая на +1 день)
		baseDate := startDate
		if nowZero.After(baseDate) {
			baseDate = nowZero
		}

		currentDate := baseDate
		for {
			currentDate = currentDate.AddDate(0, 0, 1)

			// Преобразуем стандартный Go weekday (Sunday=0, Monday=1...) в формат задания (Monday=1, Sunday=7)
			goWeekday := int(currentDate.Weekday())
			var customWeekday int
			if goWeekday == 0 {
				customWeekday = 7
			} else {
				customWeekday = goWeekday
			}

			if weekdayMap[customWeekday] && currentDate.After(nowZero) {
				return currentDate.Format(dateLayout), nil
			}
		}

	case "m":
		// === Правило дней месяца (со звездочкой) ===
		if len(tokens) < 2 {
			return "", errors.New("month days not specified")
		}

		// Парсим разрешенные дни месяца
		dayTokens := strings.Split(tokens[1], ",")
		var targetDays []int
		for _, t := range dayTokens {
			d, err := strconv.Atoi(t)
			if err != nil || d == 0 || d < -2 || d > 31 {
				return "", errors.New("invalid month day value")
			}
			targetDays = append(targetDays, d)
		}

		// Парсим разрешенные месяцы (если указаны)
		var targetMonths []int
		if len(tokens) >= 3 {
			monthTokens := strings.Split(tokens[2], ",")
			for _, t := range monthTokens {
				m, err := strconv.Atoi(t)
				if err != nil || m < 1 || m > 12 {
					return "", errors.New("invalid month value")
				}
				targetMonths = append(targetMonths, m)
			}
		}

		// Определяем начальную точку для перебора месяцев
		baseDate := startDate
		if nowZero.After(baseDate) {
			baseDate = nowZero
		}

		// Будем последовательно проверять месяцы, начиная с месяца baseDate
		currentYear := baseDate.Year()
		currentMonth := baseDate.Month()

		for iter := 0; iter < 1000; iter++ { // Защита от вечного цикла
			// Проверяем, подходит ли текущий проверяемый месяц
			if len(targetMonths) > 0 {
				monthOk := false
				for _, tm := range targetMonths {
					if int(currentMonth) == tm {
						monthOk = true
						break
					}
				}
				if !monthOk {
					// Если месяц не подошел, переходим к первому числу следующего месяца
					currentMonth++
					if currentMonth > 12 {
						currentMonth = 1
						currentYear++
					}
					continue
				}
			}

			// Для текущего года и месяца собираем все реальные даты, которые дают правила
			var candidateDates []time.Time
			for _, td := range targetDays {
				var actualDay int
				if td > 0 {
					actualDay = td
				} else {
					// Находим последний день текущего месяца через трюк с нулевым днем следующего месяца
					lastDayOfMon := time.Date(currentYear, currentMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
					actualDay = lastDayOfMon + 1 + td // td равен -1 или -2
				}

				// Проверяем валидность числа (например, 31 число в феврале отбрасываем)
				candidateDate := time.Date(currentYear, currentMonth, actualDay, 0, 0, 0, 0, nowZero.Location())
				if candidateDate.Month() == currentMonth {
					candidateDates = append(candidateDates, candidateDate)
				}
			}

			// Сортируем найденные даты-кандидаты внутри этого месяца
			sort.Slice(candidateDates, func(i, j int) bool {
				return candidateDates[i].Before(candidateDates[j])
			})

			// Ищем первую дату, которая строго больше ноля часов сегодняшнего дня (nowZero)
			for _, dCandidate := range candidateDates {
				if dCandidate.After(nowZero) && (dCandidate.After(startDate) || dCandidate.Equal(startDate)) {
					return dCandidate.Format(dateLayout), nil
				}
			}

			// Если в текущем месяце ничего не подошло, двигаемся на месяц вперед
			currentMonth++
			if currentMonth > 12 {
				currentMonth = 1
				currentYear++
			}
		}

		return "", errors.New("could not find next date within reasonable threshold")

	default:
		return "", fmt.Errorf("unsupported repeat rule type: %s", ruleType)
	}
}
