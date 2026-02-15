# Vault Search

[English](README.md) | **Русский**

Быстрый и безопасный инструмент для поиска секретов HashiCorp Vault по именам ключей и путям. Создан для SRE и DevOps инженеров, которым нужно быстро находить секреты.

## Возможности

- **Поиск по именам ключей**: Поиск по путям и именам ключей без раскрытия значений секретов
- **Извлечение вложенных ключей**: Автоматическое извлечение ключей из JSON/YAML значений в секретах
- **Регистронезависимый поиск**: Быстрый поиск подстроки через параметр `term=`
- **Поддержка регулярных выражений**: Полный контроль через параметр `regexp=` (пользователь управляет регистром)
- **Фильтрация по пути**: Фильтрация результатов по сегменту пути секрета
- **Кэш только в памяти**: Без записи на диск — секреты не попадают в файловую систему
- **Высокая производительность**: Предварительно построенные строки поиска, конкурентная загрузка, ограничение горутин
- **Защита от ReDoS**: Таймаут на поиск по регулярным выражениям

## Быстрый старт

### Docker (рекомендуется)

```bash
# Получите образ
docker pull ghcr.io/laduwka/vault-search:latest

# Запустите контейнер
docker run -d \
  --name vault-search-kv \
  -p 18080:8080 \
  -e VAULT_TOKEN="ваш-токен-vault" \
  -e VAULT_ADDR="https://vault.example.com" \
  -e VAULT_MOUNT_POINT="kv" \
  ghcr.io/laduwka/vault-search:latest
```

Для еще одного mount point:

```bash
docker run -d \
  --name vault-search-secrets \
  -p 18081:8080 \
  -e VAULT_TOKEN="ваш-токен-vault" \
  -e VAULT_ADDR="https://vault.example.com" \
  -e VAULT_MOUNT_POINT="secrets" \
  ghcr.io/laduwka/vault-search:latest
```

Проверьте работу:

```bash
# Поиск секретов с "password" в названии ключа или пути
curl "http://localhost:18080/search?term=password"

# Статус кэша
curl "http://localhost:18080/status"
```

### Коллекция API-запросов

В репозитории есть готовая коллекция HTTP-запросов в папке `opencollection/`. Совместима с [Thunder Client](https://www.thunderclient.com/) (расширение VS Code), [Bruno](https://www.usebruno.com/) и другими инструментами, поддерживающими формат Open Collection.

| Запрос | Метод | Описание |
|--------|-------|----------|
| status | GET | Статус кэша, прогресс построения |
| search term | GET | Поиск по ключевому слову с фильтром по пути |
| search regexp | GET | Поиск по регулярному выражению |
| search in_path | GET | Поиск только по пути секрета |
| rebuild | POST | Перестроение кэша |

> Все запросы настроены на `localhost:8080`. Измените адрес при необходимости.

## Установка

### Требования

- Go 1.24+
- HashiCorp Vault (KV v2 secrets engine)
- Токен Vault с правами на чтение секретов

### Сборка из исходников

```bash
git clone <repository-url>
cd vault-search
go mod tidy
go build -o vault-search .
```

### Использование Make

```bash
make build   # Сборка бинарника
make run     # Запуск приложения
make test    # Запуск тестов
make all     # tidy, fmt, vet, test, build
```

### Запуск через Docker

Локальная сборка и запуск:

```bash
# Сборка Docker-образа
docker build -t vault-search:latest .

# Запуск контейнера
docker run --rm -p 8080:8080 \
  -e VAULT_TOKEN="ваш-токен-vault" \
  -e VAULT_ADDR="https://your-vault.example.com" \
  vault-search:latest
```

Или через Make:

```bash
# Сборка Docker-образа
make docker-build

# Запуск Docker-контейнера
VAULT_TOKEN="ваш-токен" VAULT_ADDR="https://vault.example.com" make docker-run
```

### Образы из GitHub Container Registry

Готовые образы доступны в GitHub Container Registry:

```bash
# Последний образ
docker pull ghcr.io/laduwka/vault-search:latest

# Конкретная версия
docker pull ghcr.io/laduwka/vault-search:v0.2.0

# Запуск
docker run --rm -p 8080:8080 \
  -e VAULT_TOKEN="ваш-токен-vault" \
  -e VAULT_ADDR="https://your-vault.example.com" \
  ghcr.io/laduwka/vault-search:latest
```

### Проверка подписи Docker-образа

Все Docker-образы подписаны с помощью [Cosign](https://docs.sigstore.dev/cosign/overview/) (keyless signing через Sigstore) из GitHub Actions.

#### Установка Cosign

```bash
# macOS
brew install cosign

# Linux
go install github.com/sigstore/cosign/v2/cmd/cosign@latest
```

#### Проверка по тегу (рекомендуется)

Cosign автоматически разрешает тег в digest и проверяет подпись удалённо:

```bash
cosign verify \
  --certificate-identity="https://github.com/laduwka/vault-search/.github/workflows/docker.yml@refs/heads/main" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  --annotation repo=laduwka/vault-search \
  ghcr.io/laduwka/vault-search:latest
```

#### Проверка конкретной версии

```bash
cosign verify \
  --certificate-identity="https://github.com/laduwka/vault-search/.github/workflows/docker.yml@refs/tags/v0.2.0" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  --annotation repo=laduwka/vault-search \
  ghcr.io/laduwka/vault-search:v0.2.0
```

#### Проверка по digest (строгая привязка)

```bash
cosign verify \
  --certificate-identity="https://github.com/laduwka/vault-search/.github/workflows/docker.yml@refs/heads/main" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  --annotation repo=laduwka/vault-search \
  ghcr.io/laduwka/vault-search@sha256:<digest>
```

Успешная проверка покажет:
```
Verification for ghcr.io/laduwka/vault-search:latest --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The signatures were verified against the specified public key
  - Any certificates were verified against the Fulcio roots.
```

### Запуск

```bash
# Обязательная переменная
export VAULT_TOKEN="ваш-токен-vault"

# Опционально
export VAULT_ADDR="https://your-vault.example.com"
export VAULT_MOUNT_POINT="kv"
export LOCAL_SERVER_ADDR="localhost:8080"
export LOG_LEVEL="info"

# Запуск сервера
./vault-search
```

## Конфигурация

| Переменная окружения | По умолчанию | Описание |
|---------------------|--------------|----------|
| `VAULT_TOKEN` | *(обязательно)* | Токен аутентификации Vault |
| `VAULT_ADDR` | `https://your-vault.example.com` | Адрес сервера Vault |
| `VAULT_MOUNT_POINT` | `kv` | Точка монтирования KV v2 |
| `LOCAL_SERVER_ADDR` | `localhost:8080` | Адрес HTTP-сервера |
| `MAX_GOROUTINES` | `15` | Лимит конкурентных запросов к Vault |
| `LOG_LEVEL` | `info` | Уровень логирования: `debug`, `info`, `warn`, `error` |
| `LOG_FILE_PATH` | `/tmp/vault_search.log` | Путь к файлу логов (также пишет в stdout) |
| `VAULT_TIMEOUT` | `30s` | Таймаут запросов к Vault API (формат Go duration) |
| `SEARCH_TIMEOUT` | `5s` | Таймаут поисковых запросов (формат Go duration) |

## API

### Поиск секретов

```
GET /search
```

Поиск секретов по именам ключей или путям.

#### Параметры

| Параметр | Тип | Описание |
|----------|-----|----------|
| `term` | string | Регистронезависимый поиск подстроки в пути + именах ключей |
| `regexp` | string | Поиск по регулярному выражению (добавьте `(?i)` для регистронезависимого) |
| `in_path` | string | Фильтрация по сегменту пути |
| `sort` | string | Сортировка результатов: `asc` или `desc` |
| `show_ui` | boolean | Возвращать URL Vault UI вместо путей (`true`) |

**Примечание:** Требуется хотя бы один из `term`, `regexp` или `in_path`. `term` и `regexp` взаимоисключающие.

#### Ответ

```json
{
  "matches": [
    "prod/database/credentials",
    "staging/api/keys"
  ]
}
```

С `show_ui=true`:

```json
{
  "matches": [
    "https://vault.example.com/ui/vault/secrets/kv/show/prod/database/credentials",
    "https://vault.example.com/ui/vault/secrets/kv/show/staging/api/keys"
  ]
}
```

#### Примеры

```bash
# Найти все секреты с "password" в имени ключа или пути
curl "http://localhost:8080/search?term=password"

# Поиск ключей по регулярному выражению (с учётом регистра)
curl 'http://localhost:8080/search?regexp=^api_'

# Поиск ключей по регулярному выражению (без учёта регистра)
curl 'http://localhost:8080/search?regexp=(?i)^api_'

# Найти секреты в пути "prod" с "db" в именах ключей
curl "http://localhost:8080/search?term=db&in_path=prod"

# Отсортированные результаты со ссылками на Vault UI
curl "http://localhost:8080/search?term=api_key&sort=asc&show_ui=true"

# Найти секреты с "credentials" в пути
curl "http://localhost:8080/search?in_path=credentials&sort=desc"
```

### Статус кэша

```
GET /status
```

Возвращает статистику кэша и состояние перестроения.

#### Ответ

```json
{
  "cache_age": "2h 15m 30s",
  "build_duration": "45s",
  "is_rebuilding": false,
  "cache_in_mem_size": "1.2 MB",
  "fetched_secrets": 1500,
  "total_secrets": 1500,
  "total_keys_indexed": 4500,
  "progress_percentage": 100
}
```

#### Поля

| Поле | Описание |
|------|----------|
| `cache_age` | Время с последней успешной сборки кэша |
| `build_duration` | Длительность последней сборки кэша |
| `is_rebuilding` | Идёт ли перестроение |
| `cache_in_mem_size` | Оценочный размер в памяти |
| `fetched_secrets` | Количество загруженных секретов |
| `total_secrets` | Общее количество обнаруженных секретов |
| `total_keys_indexed` | Общее количество проиндексированных ключей (включая вложенные) |
| `progress_percentage` | Прогресс сборки (0-100) |

### Перестроение кэша

```
POST /rebuild
```

Запускает асинхронное перестроение кэша. Только одно перестроение может выполняться одновременно.

#### Тело запроса

```json
{
  "rebuild": "true"
}
```

#### Ответ

```json
{
  "message": "Cache rebuild started"
}
```

#### Пример

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"rebuild": "true"}' \
  "http://localhost:8080/rebuild"
```

## Как это работает

### Извлечение ключей

Инструмент извлекает имена ключей из секретов тремя способами:

1. **Ключи верхнего уровня**: Прямые ключи секрета (например, `username`, `password`)
2. **Вложенные JSON-ключи**: Парсит строковые значения JSON вида `{"host": "db", "port": 5432}`
3. **Вложенные YAML-ключи**: Парсит строковые значения YAML вида `host: db\nport: 5432`

**Эвристики для вложенного парсинга:**
- JSON: Значение начинается с `{` или `[`
- YAML: Значение содержит `:` И символ новой строки

Ошибки парсинга логируются только на уровне DEBUG.

### Построение строки поиска

Для каждого секрета создаётся предварительно построенная строка поиска:

```
"path/to/secret key1 key2 nested_key1 nested_key2 "
```

Всё в нижнем регистре для быстрого регистронезависимого поиска подстроки.

### Пример

Секрет по пути `prod/database/credentials`:

```json
{
  "username": "admin",
  "password": "secret123",
  "config": "{\"host\": \"db.example.com\", \"port\": 5432}"
}
```

Извлечённые ключи: `["username", "password", "config", "host", "port"]`

Строка поиска: `"prod/database/credentials username password config host port "`

Поиск по `term=port` или `term=PASSWORD` найдёт этот секрет.

## Безопасность

### Что кэшируется

| Кэшируется | НЕ кэшируется |
|------------|----------------|
| Пути секретов | Значения секретов |
| Имена ключей | Значения ключей |
| Имена вложенных ключей | Значения вложенных ключей |

### Функции безопасности

- **Только в памяти**: Кэш никогда не записывается на диск
- **Без раскрытия значений**: Только имена ключей доступны для поиска
- **Защита от ReDoS**: Таймаут на поиск по регулярным выражениям
- **Только локальный**: Предназначен для использования на localhost
- **Ограничение горутин**: Предотвращает исчерпание ресурсов

### Рекомендации

- Запускайте только на своей локальной машине
- Используйте токен Vault с минимально необходимыми правами
- Регулярно ротируйте токены Vault

## Производительность

### Оптимизации

| Особенность | Преимущество |
|-------------|-------------|
| Предварительно построенные строки поиска | Без маршалинга JSON при поиске |
| Конкурентная загрузка секретов | Быстрая сборка кэша |
| Семафор горутин | Контролируемая нагрузка на Vault API |
| RWMutex для кэша | Неблокирующее чтение при поиске |

### Ожидаемая производительность

| Секреты | Сборка кэша | Время поиска |
|---------|-------------|--------------|
| 100 | ~2с | <1мс |
| 1 000 | ~10с | <5мс |
| 10 000 | ~60с | <20мс |

*Время зависит от задержки Vault и настройки `MAX_GOROUTINES`*

## Разработка

### Структура проекта

```
/project/
├── main.go           # Точка входа, HTTP-сервер
├── config.go         # Конфигурация, инициализация
├── cache.go          # Управление кэшем
├── handlers.go       # HTTP-обработчики
├── search.go         # Логика поиска
├── extract.go        # Извлечение ключей
├── utils.go          # Вспомогательные функции
├── main_test.go      # Юнит-тесты
├── go.mod
└── go.sum
```

### Запуск тестов

```bash
# Запуск всех тестов
go test ./...

# Подробный вывод
go test -v ./...

# Запуск конкретного теста
go test -run TestExtractKeysFromValue ./...

# С покрытием
go test -cover ./...
```

### Качество кода

```bash
# Форматирование
go fmt ./...

# Статический анализ
go vet ./...
```

## Устранение неполадок

### Частые проблемы

#### "Failed to create Vault client"

- Проверьте правильность `VAULT_ADDR` и доступность сервера
- Проверьте сетевое подключение к Vault

#### "Access denied for secret"

- Токен не имеет прав на чтение этого пути
- Логируется на уровне WARN, секрет пропускается

#### "Search timeout exceeded"

- Слишком сложное регулярное выражение или очень большой кэш
- Упростите regex или увеличьте таймаут через переменную `SEARCH_TIMEOUT` (например, `SEARCH_TIMEOUT=10s`)

#### "Cache rebuild is already in progress"

- Только одно перестроение может выполняться одновременно
- Дождитесь завершения текущего перестроения

### Режим отладки

Включите подробное логирование:

```bash
export LOG_LEVEL=debug
./vault-search
```

Логи отладки включают:
- Загрузку каждого секрета
- Обнаружение JSON/YAML в значениях
- Ошибки парсинга вложенного контента
- Детали обхода директорий

## Лицензия

GPL-3.0

## Участие в разработке

1. Форкните репозиторий
2. Создайте feature-ветку
3. Напишите тесты для новой функциональности
4. Убедитесь, что все тесты проходят: `go test ./...`
5. Запустите линтеры: `go fmt ./... && go vet ./...`
6. Создайте pull request
