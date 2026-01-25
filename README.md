# Telegram Bots API Generator

Генератор Go-библиотеки для работы с [Telegram Bots API](https://core.telegram.org/bots/api).

Проект автоматически парсит официальную документацию Telegram и генерирует типизированные структуры и методы для работы с API ботов.

## Структура проекта

Проект состоит из двух частей:

1. **Генератор** (корневая директория) — парсер HTML-документации и генератор кода
2. **Библиотека API** (`api/`) — сгенерированная Go-библиотека для работы с Telegram Bots API

### Структура файлов генератора

```
.
├── generate.go       # Основная логика генерации кода
├── parser.go         # Парсинг HTML документации Telegram
├── helpers.go        # Вспомогательные функции для работы с HTML
├── templates/        # Шаблоны для генерации кода
│   ├── types_header.tmpl   # Заголовок файла types.go
│   ├── types.tmpl          # Шаблон для каждого типа
│   └── request.tmpl        # Шаблон для файлов запросов
└── api/              # Сгенерированная библиотека (отдельный модуль)
    ├── bot.go        # Базовая структура бота (ручной код)
    ├── constants.go  # Константы API (ручной код)
    ├── types.go      # Сгенерированные типы
    └── requests/     # Сгенерированные методы API
```

## Как работает генератор

### 1. Загрузка документации

Генератор загружает HTML-страницу с https://core.telegram.org/bots/api и парсит её структуру.

### 2. Извлечение данных

Из HTML извлекаются:

- **Типы** (Types) — структуры данных API (определяются заголовками h4 с заглавной буквы)
  - Название типа
  - Поля с типами и описанием
  - Подтипы (для полиморфных типов)

- **Методы** (Methods) — API-endpoints (определяются заголовками h4 со строчной буквы)
  - Название метода
  - Параметры (обязательные и опциональные)
  - Тип возвращаемого значения

### 3. Генерация кода

Генератор создаёт:

- `api/types.go` — все типы данных Telegram API
- `api/requests/*.go` — отдельный файл для каждого метода API

#### Специальная обработка типов

Генератор автоматически преобразует типы из документации:

| Telegram API | Go тип | Описание |
|-------------|--------|----------|
| `String` | `string` | Строка |
| `Integer` | `int64` | Целое число |
| `Float` | `float64` | Число с плавающей точкой |
| `Boolean` | `bool` | Булево значение |
| `Array of X` | `[]X` | Массив элементов типа X |
| `InputFile or String` | `InputFile` | Поле для загрузки файла |
| `Integer or String` | `ChatId` | Идентификатор чата (ID или username) |
| `X or Y or Z` | `interface{}` | Полиморфный тип (с runtime проверкой) |

#### Структура сгенерированных методов

Каждый файл в `api/requests/` содержит структуру с тремя методами:

```go
type SendMessage struct {
    ChatId telegram.ChatId
    Text   string
    // ... другие поля
}

// Call — выполняет запрос и возвращает типизированный ответ
func (r *SendMessage) Call(ctx context.Context, b *telegram.Bot) (interface{}, error)

// GetValues — конвертирует поля в map[string]interface{} для отправки
func (r *SendMessage) GetValues() (map[string]interface{}, error)

// GetFiles — извлекает файлы для multipart/form-data загрузки
func (r *SendMessage) GetFiles() map[string]io.Reader
```

### 4. Обработка InputFile

Генератор рекурсивно сканирует все типы и находит поля `InputFile`:

- В прямых полях структур
- В элементах массивов
- В вариантах полиморфных типов (union types)
- В вложенных объектах

Метод `GetFiles()` автоматически извлекает все файлы из всех вложенных структур.

## Использование

### Генерация кода

```bash
# Метод 1: Через go generate (рекомендуется)
go generate

# Метод 2: Прямой запуск
go run .
```

Команда `go generate` выполняет следующие действия (определены в `generate.go:3-5`):
1. Запускает генератор: `go run .`
2. Форматирует `api/types.go`: `gofmt -w api/types.go`
3. Форматирует `api/requests/`: `gofmt -w api/requests`

### Процесс обновления API

1. **Запустить генератор:**
   ```bash
   go generate
   ```

2. **Выполнить финальное форматирование в GoLand:**
   - Открыть директорию `api/` в GoLand
   - Выбрать Code → Reformat Code (или Ctrl+Alt+L)
   - Включить опции:
     - ✅ Include subdirectories
     - ✅ Optimize imports
     - ✅ Rearrange entries
     - ✅ Cleanup code
     - File mask(s): `*.go`
   - Нажать Run

3. **Закоммитить изменения в подмодуле:**
   ```bash
   cd api
   git add .
   git commit -m "Updated API to version X.X"
   git push
   cd ..
   ```

4. **Закоммитить изменения в основном репозитории:**
   ```bash
   git add api
   git commit -m "Updated API submodule"
   git push
   ```

### Использование сгенерированной библиотеки

Документация по использованию библиотеки доступна в репозитории: [github.com/temoon/telegram-bots-api](https://github.com/temoon/telegram-bots-api)

## Важные особенности

### Порядок полей в структурах

Поля в сгенерированных структурах упорядочены:
1. Сначала обязательные поля (required)
2. Затем опциональные поля (optional)
3. В каждой группе — по алфавиту

Опциональные поля всегда имеют тип-указатель (`*string`, `*int64`, и т.д.).

### Файлы, которые НЕ генерируются

Следующие файлы написаны вручную и **не перезаписываются** генератором:

- `api/bot.go` — базовая логика HTTP-запросов к Telegram API
- `api/constants.go` — константы
- `api/go.mod` — описание модуля

### Файлы, которые полностью перезаписываются

При каждом запуске генератора:

- `api/types.go` — **перезаписывается полностью**
- `api/requests/` — директория **удаляется и создаётся заново**

⚠️ **Не редактируйте** эти файлы вручную — все изменения будут потеряны!

## Технические детали

### Зависимости

```go
require (
    github.com/iancoleman/strcase v0.3.0  // Конвертация имён (snake_case ↔ CamelCase)
    golang.org/x/net v0.41.0              // HTML парсинг
    golang.org/x/text v0.26.0             // Работа с текстом (Title case)
)
```

### Особенности парсинга

- Методы определяются по h4-заголовкам со строчной буквы (например, `sendMessage`)
- Типы определяются по h4-заголовкам с заглавной буквы (например, `Message`)
- Тип возвращаемого значения извлекается из описания методов регулярным выражением
- Обязательность параметров:
  - Для методов: столбец "Required" в таблице
  - Для типов: отсутствие слова "Optional" в начале описания поля

### Обработка полиморфных типов

Если поле может принимать несколько типов (например, `ReplyMarkup: InlineKeyboardMarkup or ReplyKeyboardMarkup`):

- Тип поля: `interface{}`
- В `GetValues()` генерируется `switch` с проверкой типа
- Для неподдерживаемых типов возвращается ошибка

Пример:
```go
switch value := r.ReplyMarkup.(type) {
case telegram.InlineKeyboardMarkup, telegram.ReplyKeyboardMarkup:
    data, err := json.Marshal(value)
    values["reply_markup"] = string(data)
default:
    return errors.New("unsupported reply_markup field type")
}
```

## Разработка

### Структура кода генератора

- `parser.go` — HTTP-запрос и парсинг HTML
- `helpers.go` — обход DOM-дерева, извлечение текста и атрибутов
- `generate.go` — основная логика генерации:
  - `generateTypes()` — создание types.go
  - `generateRequests()` — создание файлов в requests/
  - `getGoType()` — маппинг типов Telegram → Go
  - `getInputFileFields()` — поиск полей InputFile

### Система шаблонов

Используется стандартный пакет `text/template` Go.

Переменные в шаблонах:
- `{{.Type}}` — информация о типе
- `{{.Fields}}` — список полей
- `{{.Method}}` — информация о методе
- `{{.ResponseType}}` — тип ответа
- `{{.Files}}` — карта полей с файлами

## Лицензия

См. файл LICENSE
