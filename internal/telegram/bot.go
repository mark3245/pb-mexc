package telegram

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"mexc-monitor/internal/database"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

type Bot struct {
	api          *tgbotapi.BotAPI
	db           *database.Database
	stopChan     chan struct{}
	allowedUsers map[int64]bool
}

func NewBot(token string, db *database.Database) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:          api,
		db:           db,
		stopChan:     make(chan struct{}),
		allowedUsers: make(map[int64]bool),
	}, nil
}

func (b *Bot) Start() error {
	log.Info("Запуск Telegram бота...")

	_, err := b.api.GetMe()
	if err != nil {
		return fmt.Errorf("ошибка подключения к Telegram API: %v", err)
	}

	log.Info("✅ Подключение к Telegram API установлено")

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	log.Info("✅ Канал обновлений создан, ожидание сообщений...")

	for {
		select {
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			log.Infof("Получено сообщение от пользователя %d: %s",
				update.Message.From.ID, update.Message.Text)

			if update.Message.IsCommand() {
				b.handleCommand(update.Message)
			}
		case <-b.stopChan:
			log.Info("Получен сигнал остановки бота")
			return nil
		}
	}
}

func (b *Bot) Stop() {
	close(b.stopChan)
}

func (b *Bot) handleCommand(message *tgbotapi.Message) {
	command := message.Command()
	args := message.CommandArguments()

	switch command {
	case "start":
		b.handleStartCommand(message)
	case "set":
		b.handleSetCommand(message, args)
	case "status":
		b.handleStatusCommand(message)
	case "blacklist":
		b.handleBlacklistCommand(message, args)
	case "help":
		b.handleHelpCommand(message)
	case "test":
		b.handleTestCommand(message)
	default:
		b.sendMessage(message.Chat.ID, "Неизвестная команда. Используйте /help для списка доступных команд.")
	}
}

func (b *Bot) handleSetCommand(message *tgbotapi.Message, args string) {
	parts := strings.Fields(args)
	if len(parts) != 2 {
		b.sendMessage(message.Chat.ID, "Использование: /set <параметр> <значение>\nПараметры: time, volume, change")
		return
	}

	param := parts[0]
	valueStr := parts[1]

	settings, err := b.db.GetSettings()
	if err != nil {
		log.Errorf("Failed to get settings: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка получения текущих настроек")
		return
	}

	switch param {
	case "time":
		value, err := strconv.Atoi(valueStr)
		if err != nil || value <= 0 {
			b.sendMessage(message.Chat.ID, "Неверное значение времени. Должно быть положительным целым числом.")
			return
		}
		settings.TimeInterval = value
		b.sendMessage(message.Chat.ID, fmt.Sprintf("Интервал времени установлен на %d секунд", value))

	case "volume":
		value, err := strconv.Atoi(valueStr)
		if err != nil || value <= 0 {
			b.sendMessage(message.Chat.ID, "Неверное значение объема. Должно быть положительным целым числом.")
			return
		}
		settings.MinVolume = value
		b.sendMessage(message.Chat.ID, fmt.Sprintf("Минимальный объем установлен на $%d", value))

	case "change":
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil || value <= 0 {
			b.sendMessage(message.Chat.ID, "Неверное значение изменения. Должно быть положительным числом.")
			return
		}
		settings.PriceChange = value
		b.sendMessage(message.Chat.ID, fmt.Sprintf("Порог изменения цены установлен на %.2f%%", value))

	default:
		b.sendMessage(message.Chat.ID, "Неизвестный параметр. Доступные: time, volume, change")
		return
	}

	if err := b.db.UpdateSettings(settings); err != nil {
		log.Errorf("Failed to update settings: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка сохранения настроек")
		return
	}
}

func (b *Bot) handleStatusCommand(message *tgbotapi.Message) {
	settings, err := b.db.GetSettings()
	if err != nil {
		log.Errorf("Failed to get settings: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка получения настроек")
		return
	}

	status := fmt.Sprintf("📊 Текущие настройки:\n\n"+
		"⏱ Интервал времени: %d секунд\n"+
		"📈 Изменение цены: %.2f%%\n"+
		"💰 Минимальный объем: $%d\n",
		settings.TimeInterval, settings.PriceChange, settings.MinVolume)

	b.sendMessage(message.Chat.ID, status)
}

func (b *Bot) handleBlacklistCommand(message *tgbotapi.Message, args string) {
	parts := strings.Fields(args)

	if len(parts) == 0 {
		entries, err := b.db.GetBlacklist()
		if err != nil {
			log.Errorf("Failed to get blacklist: %v", err)
			b.sendMessage(message.Chat.ID, "Error getting blacklist")
			return
		}

		if len(entries) == 0 {
			b.sendMessage(message.Chat.ID, "Черный список пуст")
			return
		}

		var response strings.Builder
		response.WriteString("🚫 Черный список:\n\n")
		for _, entry := range entries {
			remaining := time.Until(entry.ExpiresAt)
			response.WriteString(fmt.Sprintf("• %s (истекает через %s)\n",
				entry.Symbol, formatDuration(remaining)))
		}
		b.sendMessage(message.Chat.ID, response.String())
		return
	}

	if len(parts) != 2 {
		b.sendMessage(message.Chat.ID, "Использование: /blacklist <символ> <длительность_в_секундах>\nПример: /blacklist BTC 3600")
		return
	}

	symbol := strings.ToUpper(parts[0])
	durationStr := parts[1]

	duration, err := strconv.Atoi(durationStr)
	if err != nil || duration <= 0 {
		b.sendMessage(message.Chat.ID, "Неверная длительность. Должно быть положительным целым числом (секунды).")
		return
	}

	if err := b.db.AddToBlacklist(symbol, time.Duration(duration)*time.Second); err != nil {
		log.Errorf("Failed to add to blacklist: %v", err)
		b.sendMessage(message.Chat.ID, "Ошибка добавления в черный список")
		return
	}

	b.sendMessage(message.Chat.ID, fmt.Sprintf("Добавлено %s в черный список на %s",
		symbol, formatDuration(time.Duration(duration)*time.Second)))
}

func (b *Bot) handleStartCommand(message *tgbotapi.Message) {
	b.AddUser(message.Chat.ID)

	welcomeMsg := `🤖 Добро пожаловать в MEXC Monitor Bot!

Этот бот отслеживает цены и объемы криптовалют на бирже MEXC и отправляет уведомления при значительных изменениях.

Доступные команды:
• /start - Запустить бота и получать алерты
• /status - Показать текущие настройки
• /set time (секунды) - Установить интервал мониторинга
• /set volume (сумма) - Установить минимальный объем
• /set change (процент) - Установить порог изменения цены
• /blacklist (символ) (секунды) - Добавить монету в черный список
• /blacklist - Показать черный список
• /help - Показать справку
• /test - Отправить тестовый алерт

Примеры:
/set time 5
/set volume 5000
/set change 2.5`

	b.sendMessage(message.Chat.ID, welcomeMsg)

	go func() {
		time.Sleep(2 * time.Second)
		b.SendAlert("TEST/USDT", 2.5, 15000, time.Now())
	}()
}

func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	helpMsg := `📋 Команды MEXC Monitor Bot:

🔧 Настройки:
• /set time (секунды) - Установить интервал мониторинга (по умолчанию: 5)
• /set volume (сумма) - Установить минимальный объем в USD (по умолчанию: 5000)
• /set change (процент) - Установить порог изменения цены (по умолчанию: 2.0)

📊 Информация:
• /status - Показать текущие настройки
• /blacklist - Показать черный список монет

🚫 Управление черным списком:
• /blacklist (символ) (секунды) - Добавить монету в черный список на указанное время
• Пример: /blacklist BTC 3600 (заблокировать BTC на 1 час)

📈 Алерты:
Алерты отправляются когда:
- Цена изменяется на указанный процент в течение интервала времени
- Объем торгов превышает минимальный порог
- Монета не находится в черном списке

Примеры использования:
/set time 10
/set volume 10000
/set change 3.0
/blacklist DOGE 1800`

	b.sendMessage(message.Chat.ID, helpMsg)
}

func (b *Bot) handleTestCommand(message *tgbotapi.Message) {
	b.sendMessage(message.Chat.ID, "🧪 Отправка тестового алерта...")

	if err := b.SendAlert("TEST/USDT", 2.5, 15000, time.Now()); err != nil {
		b.sendMessage(message.Chat.ID, "❌ Не удалось отправить тестовый алерт")
	} else {
		b.sendMessage(message.Chat.ID, "✅ Тестовый алерт отправлен успешно!")
	}
}

func (b *Bot) SendAlert(symbol string, priceChange float64, volume int, timestamp time.Time) error {
	message := formatAlertMessage(symbol, priceChange, volume, timestamp)

	log.Infof("Отправка алерта %d пользователям", len(b.allowedUsers))

	for userID := range b.allowedUsers {
		msg := tgbotapi.NewMessage(userID, message)
		msg.ParseMode = "HTML"

		if _, err := b.api.Send(msg); err != nil {
			log.Errorf("Не удалось отправить алерт пользователю %d: %v", userID, err)
		} else {
			log.Infof("Успешно отправлен алерт пользователю %d", userID)
		}
	}

	if len(b.allowedUsers) == 0 {
		log.Warn("Нет пользователей в списке разрешенных. Отправьте /start боту сначала!")
	}

	return nil
}

func (b *Bot) AddUser(userID int64) {
	b.allowedUsers[userID] = true
	log.Infof("Добавлен пользователь %d в список разрешенных", userID)
}

func (b *Bot) RemoveUser(userID int64) {
	delete(b.allowedUsers, userID)
	log.Infof("Удален пользователь %d из списка разрешенных", userID)
}

func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"

	if _, err := b.api.Send(msg); err != nil {
		log.Errorf("Failed to send message: %v", err)
	}
}

func formatAlertMessage(symbol string, priceChange float64, volume int, timestamp time.Time) string {
	priceChangeStr := fmt.Sprintf("%.2f%%", priceChange)
	if priceChange > 0 {
		priceChangeStr = "+" + priceChangeStr
	}

	volumeStr := formatVolume(volume)

	volumeEmojis := getVolumeEmojis(volume)
	priceEmojis := getPriceEmojis(priceChange)

	timeStr := timestamp.Format("15:04:05")

	return fmt.Sprintf("⚡ <b>ALERT</b>\n\n"+
		"<b>%s</b>\n\n"+
		"📈 <b>Изменение цены:</b> %s %s\n"+
		"💰 <b>Объём торгов:</b> %s %s\n"+
		"⏰ <b>Время:</b> %s",
		symbol, priceChangeStr, priceEmojis, volumeStr, volumeEmojis, timeStr)
}

func formatVolume(volume int) string {
	if volume >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(volume)/1000000)
	} else if volume >= 1000 {
		return fmt.Sprintf("%.1fK", float64(volume)/1000)
	}
	return fmt.Sprintf("%d", volume)
}

func getVolumeEmojis(volume int) string {
	if volume < 10000 {
		return ""
	} else if volume < 50000 {
		return "👁"
	} else if volume < 100000 {
		return "👁🔥"
	} else if volume < 150000 {
		return "👁🔥🔥"
	} else if volume < 200000 {
		return "👁🔥🔥🔥"
	} else {
		fireCount := (volume-200000)/50000 + 3
		if fireCount > 10 {
			fireCount = 10
		}
		fires := strings.Repeat("🔥", fireCount)
		return "👁" + fires
	}
}

func getPriceEmojis(priceChange float64) string {
	change := math.Abs(priceChange)

	circleCount := int(change/10) + 1
	if circleCount > 10 {
		circleCount = 10
	}

	return strings.Repeat("🔵", circleCount)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}
