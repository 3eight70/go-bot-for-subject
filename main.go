// Package main implements a Telegram conversation bot rewritten from Python to Go.
// Хранение данных переведено на in-memory (никаких БД/файлов).
//
// ==================== IMPORTANT ====================
// API токен задаётся через переменную окружения TELEGRAM_BOT_TOKEN.
// Пример: export TELEGRAM_BOT_TOKEN="123456:ABC..."
// В Docker: см. Dockerfile и docker-compose.yml
// ================================================

package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	StateChoosing = iota
	StateTypingReply
	StateTypingChoice
)

// Sender — минимальный интерфейс для отправки сообщений (удобно мокать в тестах).
type Sender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// UserSession хранит состояние пользователя.
type UserSession struct {
	State         int
	Data          map[string]string // category -> value (храним в нижнем регистре)
	PendingChoice string
}

// SessionStore — потокобезопасное in-memory хранилище сессий.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[int64]*UserSession // key = Telegram user ID
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[int64]*UserSession),
	}
}

func (s *SessionStore) Get(userID int64) *UserSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[userID]; ok {
		return sess
	}
	ns := &UserSession{
		State: StateChoosing,
		Data:  make(map[string]string),
	}
	s.sessions[userID] = ns
	return ns
}

func factsToStr(data map[string]string) string {
	if len(data) == 0 {
		return "\n\n"
	}
	lines := make([]string, 0, len(data))
	for k, v := range data {
		lines = append(lines, fmt.Sprintf("%s - %s", k, v))
	}
	return "\n" + strings.Join(lines, "\n") + "\n"
}

func makeKeyboard() tgbotapi.ReplyKeyboardMarkup {
	kbd := tgbotapi.NewReplyKeyboard(
		[]tgbotapi.KeyboardButton{
			tgbotapi.NewKeyboardButton("Age"),
			tgbotapi.NewKeyboardButton("Favourite colour"),
		},
		[]tgbotapi.KeyboardButton{
			tgbotapi.NewKeyboardButton("Number of siblings"),
			tgbotapi.NewKeyboardButton("Something else..."),
		},
		[]tgbotapi.KeyboardButton{
			tgbotapi.NewKeyboardButton("Done"),
		},
	)
	kbd.OneTimeKeyboard = true
	return kbd
}

func replyWithKeyboard(sender Sender, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	kbd := makeKeyboard()
	msg.ReplyMarkup = kbd
	_, _ = sender.Send(msg)
}

func handleStart(sender Sender, update tgbotapi.Update, sess *UserSession) {
	chatID := update.Message.Chat.ID
	reply := "Hi! My name is Doctor Botter."
	if len(sess.Data) > 0 {
		keys := make([]string, 0, len(sess.Data))
		for k := range sess.Data {
			keys = append(keys, k)
		}
		reply += fmt.Sprintf(" You already told me your %s. Why don't you tell me something more about yourself? Or change anything I already know.", strings.Join(keys, ", "))
	} else {
		reply += " I will hold a more complex conversation with you. Why don't you tell me something about yourself?"
	}
	sess.State = StateChoosing
	sess.PendingChoice = ""
	replyWithKeyboard(sender, chatID, reply)
}

func handleShowData(sender Sender, update tgbotapi.Update, sess *UserSession) {
	chatID := update.Message.Chat.ID
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("This is what you already told me: %s", factsToStr(sess.Data)))
	_, _ = sender.Send(msg)
}

func handleDone(sender Sender, update tgbotapi.Update, sess *UserSession) {
	chatID := update.Message.Chat.ID
	sess.PendingChoice = ""
	sess.State = StateChoosing
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("I learned these facts about you: %sUntil next time!", factsToStr(sess.Data)))
	msg.ReplyMarkup = tgbotapi.ReplyKeyboardRemove{RemoveKeyboard: true}
	_, _ = sender.Send(msg)
}

func handleCustomChoice(sender Sender, update tgbotapi.Update, sess *UserSession) {
	chatID := update.Message.Chat.ID
	sess.State = StateTypingChoice
	msg := tgbotapi.NewMessage(chatID, `Alright, please send me the category first, for example "Most impressive skill"`)
	_, _ = sender.Send(msg)
}

func handleRegularChoice(sender Sender, update tgbotapi.Update, sess *UserSession, choice string) {
	chatID := update.Message.Chat.ID
	lc := strings.ToLower(choice)
	sess.PendingChoice = lc
	sess.State = StateTypingReply

	var reply string
	if v, ok := sess.Data[lc]; ok && v != "" {
		reply = fmt.Sprintf("Your %s? I already know the following about that: %s", lc, v)
	} else {
		reply = fmt.Sprintf("Your %s? Yes, I would love to hear about that!", lc)
	}
	msg := tgbotapi.NewMessage(chatID, reply)
	_, _ = sender.Send(msg)
}

func handleCategoryName(sender Sender, update tgbotapi.Update, sess *UserSession, category string) {
	chatID := update.Message.Chat.ID
	lc := strings.ToLower(strings.TrimSpace(category))
	if lc == "" {
		msg := tgbotapi.NewMessage(chatID, "Please provide a non-empty category name.")
		_, _ = sender.Send(msg)
		return
	}
	sess.PendingChoice = lc
	sess.State = StateTypingReply

	var reply string
	if v, ok := sess.Data[lc]; ok && v != "" {
		reply = fmt.Sprintf("Your %s? I already know the following about that: %s", lc, v)
	} else {
		reply = fmt.Sprintf("Your %s? Yes, I would love to hear about that!", lc)
	}
	msg := tgbotapi.NewMessage(chatID, reply)
	_, _ = sender.Send(msg)
}

func handleReceivedInformation(sender Sender, update tgbotapi.Update, sess *UserSession, value string) {
	chatID := update.Message.Chat.ID
	val := strings.ToLower(strings.TrimSpace(value))
	category := sess.PendingChoice
	if category == "" {
		sess.State = StateChoosing
		replyWithKeyboard(sender, chatID, "Let's start again. Please pick a category.")
		return
	}
	sess.Data[category] = val
	sess.PendingChoice = ""
	sess.State = StateChoosing

	msg := tgbotapi.NewMessage(
		chatID,
		fmt.Sprintf("Neat! Just so you know, this is what you already told me:%sYou can tell me more, or change your opinion on something.", factsToStr(sess.Data)),
	)
	kbd := makeKeyboard()
	msg.ReplyMarkup = kbd
	_, _ = sender.Send(msg)
}

func main() {
	// ==================== IMPORTANT ====================
	// Здесь бот читает токен из переменной окружения TELEGRAM_BOT_TOKEN
	// export TELEGRAM_BOT_TOKEN="ваш_токен"
	// ===================================================
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("failed to create bot: %v", err)
	}
	bot.Debug = false
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)
	store := NewSessionStore()

	for update := range updates {
		if update.Message == nil {
			continue
		}
		// Safety: если From пуст, используем chatID.
		var userID int64
		if update.Message.From != nil {
			userID = update.Message.From.ID
		} else {
			userID = update.Message.Chat.ID
		}
		sess := store.Get(userID)

		text := update.Message.Text
		if text == "" {
			continue
		}

		// Commands
		switch {
		case strings.HasPrefix(text, "/start"):
			handleStart(bot, update, sess)
			continue
		case strings.HasPrefix(text, "/show_data"):
			handleShowData(bot, update, sess)
			continue
		}

		// Non-command messages
		switch {
		case text == "Done":
			handleDone(bot, update, sess)
		case text == "Something else...":
			handleCustomChoice(bot, update, sess)
		case text == "Age" || text == "Favourite colour" || text == "Number of siblings":
			handleRegularChoice(bot, update, sess, text)
		default:
			// Depends on state
			switch sess.State {
			case StateTypingChoice:
				handleCategoryName(bot, update, sess, text)
			case StateTypingReply:
				handleReceivedInformation(bot, update, sess, text)
			default:
				replyWithKeyboard(bot, update.Message.Chat.ID, "Please pick an option or use 'Something else...' to add your own.")
			}
		}
	}
}
