package candidates

import "time"

// Competency represents a single skill assessment
type Competency struct {
	Name    string `json:"name"`
	Score   int    `json:"score"`   // 1-8
	Comment string `json:"comment"`
}

// Candidate represents an interview result
type Candidate struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Date         time.Time    `json:"date"`
	Type         string       `json:"type"` // "manual" or "automation"
	Conclusion   string       `json:"conclusion"`
	Result       string       `json:"result"` // "accepted", "rejected", "accepted_no_sb"
	Competencies []Competency `json:"competencies"`
	AvgScore     float64      `json:"avg_score"`
	Level        string       `json:"level"` // Junior, Junior+, Middle, Middle+, Senior, etc.
	Grade        int          `json:"grade"` // 8-15
}

// Stats holds aggregated interview statistics
type Stats struct {
	Total      int     `json:"total"`
	Accepted   int     `json:"accepted"`
	Rejected   int     `json:"rejected"`
	NoSB       int     `json:"no_sb"` // passed interview but failed security check
	Conversion float64 `json:"conversion"` // accepted / total * 100
	AvgScore   float64 `json:"avg_score"`
}

// Response is the API response for candidates page
type Response struct {
	Candidates []Candidate `json:"candidates"`
	Stats      Stats       `json:"stats"`
	Conclusion string      `json:"conclusion"`
}

// DefaultCompetencies is the list of competencies used in interviews (legacy, kept for backward compat)
var DefaultCompetencies = []string{
	"Способы интеграции систем",
	"Проектирование интеграций (SOAP, REST, gRPC, очереди)",
	"Описание ТЗ на разработку",
	"Проектирование БД (SQL, связи, индексы, нормализация)",
	"Синхронное/асинхронное взаимодействие",
	"Брокеры сообщений (Kafka/RabbitMQ)",
	"Разбор архитектуры (монолит/микросервис/SOA)",
}

// CompetencyDef defines a competency with its max score
type CompetencyDef struct {
	Name     string `json:"name"`
	MaxScore int    `json:"max_score"`
}

// AutomationCompetencies — навыки для automation тестировщиков
var AutomationCompetencies = []CompetencyDef{
	// 7-балльная шкала
	{"Техники тестирования и тест-дизайна", 7},
	{"Классификация и виды тестирования", 7},
	{"Тестовая документация", 7},
	{"SQL / Работа с БД", 7},
	{"Клиент-сервер", 7},
	{"STLC / SDLC", 7},
	{"Работа с логами", 7},
	{"Test Strategy Basics", 7},
	{"Test Management Basics", 7},
	{"Эвристические подходы", 7},
	{"Python Core", 7},
	{"UI Automation (Selenium)", 7},
	{"API Automation", 7},
	{"HTTP / REST", 7},
	{"Page Object Pattern", 7},
	{"CI/CD", 7},
	{"Allure Reports", 7},
	{"Docker", 7},
	{"Performance Testing (k6)", 7},
	{"Управление командой", 7},
	{"Mentoring / Onboarding", 7},
	{"Code Review", 7},
	{"Проведение интервью", 7},
	{"Конфликтные ситуации", 7},
	{"Коммуникация с разработкой", 7},
	{"Приоритизация задач", 7},
	{"Flaky tests", 7},
	{"Regression Planning", 7},
	{"Automation Framework Design", 7},
	{"CI/CD Pipeline Scenario", 7},
	{"Bug prioritization", 7},
	// 3-балльная шкала
	{"JIRA", 3},
	{"Confluence", 3},
	{"Postman", 3},
	{"TestRail / TestOps", 3},
	{"Kibana / Graylog", 3},
	{"Git", 3},
	// 4-балльная шкала
	{"Коммуникация", 4},
	{"Системное мышление", 4},
	{"Самостоятельность", 4},
	{"Стрессоустойчивость", 4},
}

// ManualCompetencies — навыки для manual тестировщиков
var ManualCompetencies = []CompetencyDef{
	// 7-балльная шкала
	{"Техники тестирования и тест-дизайна", 7},
	{"Классификация и виды тестирования", 7},
	{"Тестовая документация (тест-кейсы, чек-листы, баг-репорты)", 7},
	{"STLC / SDLC", 7},
	{"Test Strategy Basics", 7},
	{"Test Management Basics", 7},
	{"Эвристические подходы", 7},
	{"Exploratory Testing", 7},
	{"Risk-Based Testing", 7},
	{"SQL / Работа с БД", 7},
	{"Клиент-сервер", 7},
	{"HTTP / REST (понимание запросов, статус-кодов)", 7},
	{"Работа с логами", 7},
	{"Работа с DevTools (Chrome/Safari)", 7},
	{"Postman (базовая проверка API)", 7},
	{"Мобильное тестирование (Android/iOS)", 7},
	{"Кросс-браузерное / кросс-платформенное тестирование", 7},
	{"Локализация и интернационализация", 7},
	{"Accessibility Testing (WCAG)", 7},
	{"Управление командой", 7},
	{"Mentoring / Onboarding", 7},
	{"Проведение интервью", 7},
	{"Конфликтные ситуации", 7},
	{"Коммуникация с разработкой", 7},
	{"Приоритизация задач", 7},
	{"Планирование регрессии", 7},
	{"Управление тестовыми данными", 7},
	{"Координация релизов", 7},
	{"Bug prioritization / severity", 7},
	{"Regression Planning", 7},
	{"Test Coverage Analysis", 7},
	{"Release Readiness Assessment", 7},
	{"Воспроизведение сложных багов", 7},
	{"Тестирование интеграций", 7},
	{"Smoke / Sanity стратегия", 7},
	// 3-балльная шкала
	{"JIRA", 3},
	{"Confluence", 3},
	{"TestRail / TestOps", 3},
	{"Kibana / Graylog", 3},
	{"Git (базовый уровень)", 3},
	{"Charles / Fiddler (сниффинг трафика)", 3},
	{"Figma (сверка с макетами)", 3},
	{"BrowserStack / устройства", 3},
	// 4-балльная шкала
	{"Коммуникация", 4},
	{"Системное мышление", 4},
	{"Внимательность к деталям", 4},
	{"Самостоятельность", 4},
	{"Стрессоустойчивость", 4},
	{"Аналитическое мышление", 4},
}

// LevelFromScore calculates level from average score
func LevelFromScore(avg float64) (string, int) {
	switch {
	case avg >= 8:
		return "Teamlead+", 8
	case avg >= 7:
		return "Teamlead", 9
	case avg >= 6:
		return "Senior+", 10
	case avg >= 5:
		return "Senior", 11
	case avg >= 4:
		return "Middle+", 12
	case avg >= 3:
		return "Middle", 13
	case avg >= 2:
		return "Junior+", 14
	default:
		return "Junior", 15
	}
}
