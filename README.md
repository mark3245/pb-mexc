# MEXC Monitor

Автоматический мониторинг спотовых пар на бирже MEXC с Telegram-уведомлениями.

## Функциональность

- 🔍 Мониторинг всех спотовых пар MEXC в реальном времени
- ⚡ Автоматические алерты при изменении цены и объема
- 📱 Управление через Telegram-бот
- ⚙️ Настраиваемые параметры мониторинга
- 🚫 Черный список монет с временными ограничениями
- 💾 Сохранение настроек в SQLite базе данных

## Условия для алертов

Алерт срабатывает, когда:
- Цена монеты изменилась на заданный процент за указанный период времени
- Объем торгов за этот период превышает минимальный порог
- Монета не находится в черном списке

## Установка

### Требования

- Go 1.21+
- SQLite3
- Telegram Bot Token
- Telegram Channel ID

### 1. Клонирование репозитория

```bash
git clone <repository-url>
cd mexc-monitor
```

### 2. Настройка конфигурации

Отредактируйте файл `config.yaml`:

```yaml
telegram:
  bot_token: "YOUR_BOT_TOKEN_HERE"

mexc:
  websocket_url: "wss://wbs.mexc.com/ws"

monitoring:
  time_interval: 5        # секунды
  price_change: 2.0       # процент
  min_volume: 5000        # USD

database:
  path: "data/monitor.db"

logging:
  level: "info"
  file: "logs/monitor.log"
```

### 3. Создание Telegram бота

1. Найдите @BotFather в Telegram
2. Создайте нового бота: `/newbot`
3. Получите токен бота
4. Начните чат с ботом и отправьте `/start`

### 4. Запуск

#### Вариант 1: Локальный запуск

```bash
# Установка зависимостей
go mod download

# Создание директорий
mkdir -p data logs

# Запуск
go run main.go
```

#### Вариант 2: Docker

```bash
# Сборка и запуск
docker-compose up -d

# Просмотр логов
docker-compose logs -f
```

#### Вариант 3: Systemd сервис

```bash
# Создание пользователя
sudo useradd -r -s /bin/false mexc-monitor

# Копирование файлов
sudo mkdir -p /opt/mexc-monitor
sudo cp mexc-monitor /opt/mexc-monitor/
sudo cp config.yaml /opt/mexc-monitor/
sudo cp mexc-monitor.service /etc/systemd/system/

# Установка прав
sudo chown -R mexc-monitor:mexc-monitor /opt/mexc-monitor
sudo chmod +x /opt/mexc-monitor/mexc-monitor

# Запуск сервиса
sudo systemctl daemon-reload
sudo systemctl enable mexc-monitor
sudo systemctl start mexc-monitor
```

## Управление через Telegram

### Команды

- `/start` - начать работу с ботом и получать алерты
- `/help` - показать справку по командам
- `/set time 10` - установить интервал анализа 10 секунд
- `/set volume 10000` - установить минимальный объем $10,000
- `/set change 3` - установить порог изменения цены 3%
- `/status` - показать текущие настройки
- `/blacklist` - показать черный список
- `/blacklist BTC 3600` - добавить BTC в черный список на 1 час

### Примеры использования

```
/set time 5
/set volume 5000
/set change 2.5
/blacklist DOGE 1800
/status
```

## Формат уведомлений

Уведомления содержат:
- Название торговой пары
- Процент изменения цены
- Объем торгов в USD
- Время срабатывания
- Эмодзи для визуального оформления

### Эмодзи для объема:
- 10k-49k: 👁
- 50k-99k: 👁🔥
- 100k-149k: 👁🔥🔥
- 150k-199k: 👁🔥🔥🔥
- Далее +1🔥 каждые 50k (максимум 10)

### Эмодзи для изменения цены:
- 0-9%: 🔵
- 10-19%: 🔵🔵
- 20-29%: 🔵🔵🔵
- И так далее (максимум 10)

### Пример уведомления:

```
⚡ ALERT
BTC/USDT
Цена: +3.5% 🔵🔵
Объём: 12,300$ 👁🔥
Время: 20:15:04
```

## Структура проекта

```
mexc-monitor/
├── main.go                 # Главный файл приложения
├── config.yaml            # Конфигурация
├── go.mod                 # Go модуль
├── Dockerfile             # Docker образ
├── docker-compose.yml     # Docker Compose
├── mexc-monitor.service   # Systemd сервис
├── internal/
│   ├── config/           # Конфигурация
│   ├── database/         # База данных
│   ├── mexc/            # MEXC API клиент
│   ├── monitor/         # Основная логика мониторинга
│   └── telegram/        # Telegram бот
├── data/                # База данных SQLite
└── logs/                # Логи приложения
```

## Мониторинг и логи

### Логи

Логи сохраняются в файл `logs/monitor.log` и содержат:
- Информацию о подключении к MEXC
- Обработку торговых данных
- Отправку уведомлений
- Ошибки и предупреждения

### Проверка статуса

```bash
# Docker
docker-compose ps

# Systemd
sudo systemctl status mexc-monitor

# Логи
tail -f logs/monitor.log
```

## Безопасность

- Приложение работает с правами ограниченного пользователя
- Настройки хранятся в локальной SQLite базе
- Telegram токен хранится в конфигурационном файле
- Поддержка HTTPS для WebSocket соединений

## Устранение неполадок

### Проблемы с подключением к MEXC

1. Проверьте интернет-соединение
2. Убедитесь, что WebSocket URL корректный
3. Проверьте логи на наличие ошибок

### Проблемы с Telegram

1. Проверьте корректность токена бота
2. Убедитесь, что бот добавлен в канал
3. Проверьте ID канала

### Проблемы с базой данных

1. Проверьте права доступа к директории `data/`
2. Убедитесь, что SQLite установлен
3. Проверьте свободное место на диске

## Лицензия

MIT License

## Поддержка

При возникновении проблем создайте issue в репозитории проекта.
