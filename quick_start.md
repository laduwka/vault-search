# Быстрый старт

Это руководство поможет вам запустить **vault-search** на вашем ноутбуке за пару минут.

## Требования

- Docker (рекомендуется) ИЛИ Go 1.24+
- Токен от HashiCorp Vault с правами на чтение секретов

---

## Способ 1: Docker (рекомендуется)

### Шаг 1: Получите образ

```bash
# Из GitHub Container Registry (готовый образ)
docker pull ghcr.io/laduwka/vault-search:latest
```

### (Опционально) Проверка подписи образа

Все Docker-образы подписаны с помощью [Cosign](https://docs.sigstore.dev/cosign/overview/). Вы можете проверить подлинность образа перед запуском:

```bash
# Установите cosign (macOS)
brew install cosign

# Или Linux
go install github.com/sigstore/cosign/v2/cmd/cosign@latest

# Проверьте подпись
cosign verify \
  --certificate-identity-regexp="^https://github.com/laduwka/vault-search/.github/workflows/" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/laduwka/vault-search:latest
```

Успешная проверка покажет:
```
Verification for ghcr.io/laduwka/vault-search:latest --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The signatures were verified against the specified public key
  - Any certificates were verified against the Fulcio roots.
```

### Шаг 2: Запустите контейнер

```bash
docker run --rm -p 8080:8080 \
  -e VAULT_TOKEN="ваш-токен-vault" \
  -e VAULT_ADDR="https://vault.example.com" \
  ghcr.io/laduwka/vault-search:latest
```

### Шаг 3: Используйте

Откройте браузер или выполните запрос:

```bash
# Поиск секретов с "password" в названии ключа или пути
curl "http://localhost:8080/search?term=password"

# Получить статус кэша
curl "http://localhost:8080/status"
```

---

## Способ 2: Бинарный файл

### Шаг 1: Скачайте бинарник

Со страницы релизов на GitHub или соберите сами:

```bash
# Клонируйте репозиторий
git clone https://github.com/laduwka/vault-search.git
cd vault-search

# Соберите бинарник
go build -o vault-search .
```

### Шаг 2: Настройте переменные окружения

```bash
# Обязательно
export VAULT_TOKEN="ваш-токен-vault"

# Опционально (с значениями по умолчанию)
export VAULT_ADDR="https://vault.example.com"
export VAULT_MOUNT_POINT="kv"
export LOCAL_SERVER_ADDR="localhost:8080"
export LOG_LEVEL="info"
```

### Шаг 3: Запустите

```bash
./vault-search
```

---

## Примеры использования

### Поиск по названию ключа (без учёта регистра)

```bash
curl "http://localhost:8080/search?term=api_key"
```

### Поиск по регулярному выражению

```bash
# Чувствительный к регистру
curl "http://localhost:8080/search?regexp=^db_"

# Без учёта регистра (добавьте (?i))
curl "http://localhost:8080/search?regexp=(?i)^db_"
```

### Фильтр по пути

```bash
curl "http://localhost:8080/search?in_path=prod"
```

### Комбинированный поиск

```bash
# Ключи "password" в путях содержащих "prod"
curl "http://localhost:8080/search?term=password&in_path=prod"
```

### Получить ссылки на Vault UI

```bash
curl "http://localhost:8080/search?term=password&show_ui=true"
```

### Сортировка результатов

```bash
curl "http://localhost:8080/search?term=password&sort=asc"
```

---

## Проверка статуса

```bash
curl "http://localhost:8080/status"
```

Ответ:
```json
{
  "cache_age": "2h 15m",
  "is_rebuilding": false,
  "total_secrets": 1500,
  "total_keys_indexed": 4500,
  "progress_percentage": 100
}
```

---

## Перезагрузка кэша

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"rebuild": "true"}' \
  "http://localhost:8080/rebuild"
```

---

## Коллекция API-запросов

В репозитории есть готовая коллекция HTTP-запросов в папке `opencollection/`. Она совместима с [Thunder Client](https://www.thunderclient.com/) (расширение VS Code) и другими инструментами, поддерживающими формат Open Collection.

### Импорт в Thunder Client

1. Установите расширение **Thunder Client** в VS Code
2. Откройте Thunder Client на боковой панели
3. Нажмите на меню коллекций → **Import** → выберите файл `opencollection/opencollection.yml`

### Содержимое коллекции

| Запрос | Метод | Описание |
|--------|-------|----------|
| status | GET | Статус кэша, прогресс построения |
| search term | GET | Поиск по ключевому слову с фильтром по пути |
| search regexp | GET | Поиск по регулярному выражению |
| search in_path | GET | Поиск только по пути секрета |
| rebuild | POST | Перестроение кэша |

> Все запросы настроены на `localhost:8080`. Измените адрес в параметрах запроса при необходимости.

---

## Полезные советы

1. **Безопасность**: Кэш хранится только в памяти, секреты не попадают на диск
2. **Производительность**: При первом запуске кэш строится в фоне, поиск может вернуть пустой результат
3. **Логирование**: Установите `LOG_LEVEL=debug` для детальной диагностики
4. **Порт**: Если порт 8080 занят, укажите другой через `LOCAL_SERVER_ADDR`

---

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|--------------|----------|
| `VAULT_TOKEN` | *(обязательно)* | Токен авторизации Vault |
| `VAULT_ADDR` | `https://vault.offline.shelopes.com` | Адрес Vault сервера |
| `VAULT_MOUNT_POINT` | `kv` | Точка монтирования KV v2 |
| `LOCAL_SERVER_ADDR` | `localhost:8080` | Адрес для HTTP сервера |
| `MAX_GOROUTINES` | `15` | Лимит конкурентных запросов к Vault |
| `LOG_LEVEL` | `info` | Уровень логирования: debug, info, warn, error |
| `LOG_FILE_PATH` | `/tmp/vault_search.log` | Путь к файлу логов |
