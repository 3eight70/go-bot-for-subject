package main

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// fakeBot implements Sender and collects messages instead of sending them.
type fakeBot struct {
	sent []tgbotapi.Chattable
}

func (f *fakeBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.sent = append(f.sent, c)
	return tgbotapi.Message{}, nil
}

func TestFactsToStr(t *testing.T) {
	data := map[string]string{"age": "25", "favourite colour": "blue"}
	out := factsToStr(data)
	if len(out) == 0 || out[0] != '\n' || out[len(out)-1] != '\n' {
		t.Fatalf("factsToStr should be wrapped with newlines, got: %q", out)
	}
}

func TestFlow_RegularChoiceAndValue(t *testing.T) {
	sess := &UserSession{State: StateChoosing, Data: map[string]string{}}
	b := &fakeBot{}
	chatID := int64(1)

	// simulate /start
	update := tgbotapi.Update{Message: &tgbotapi.Message{
		Text: "/start",
		Chat: &tgbotapi.Chat{ID: chatID},
		From: &tgbotapi.User{ID: 123},
	}}
	handleStart(b, update, sess) // uses replyWithKeyboard, теперь не падает
	if sess.State != StateChoosing {
		t.Fatalf("expected StateChoosing after /start")
	}

	// choose "Age"
	update.Message.Text = "Age"
	handleRegularChoice(b, update, sess, "Age")
	if sess.State != StateTypingReply || sess.PendingChoice != "age" {
		t.Fatalf("expected StateTypingReply with pending 'age', got state=%d, pending=%q", sess.State, sess.PendingChoice)
	}

	// send value
	update.Message.Text = "25"
	handleReceivedInformation(b, update, sess, "25")
	if sess.State != StateChoosing {
		t.Fatalf("expected StateChoosing after saving info")
	}
	if sess.Data["age"] != "25" {
		t.Fatalf("expected age stored as 25, got %q", sess.Data["age"])
	}
}

func TestFlow_CustomCategory(t *testing.T) {
	sess := &UserSession{State: StateChoosing, Data: map[string]string{}}
	b := &fakeBot{}
	update := tgbotapi.Update{Message: &tgbotapi.Message{
		Text: "Something else...",
		Chat: &tgbotapi.Chat{ID: 1},
		From: &tgbotapi.User{ID: 42},
	}}
	handleCustomChoice(b, update, sess)
	if sess.State != StateTypingChoice {
		t.Fatalf("expected StateTypingChoice")
	}

	update.Message.Text = "Most impressive skill"
	handleCategoryName(b, update, sess, update.Message.Text)
	if sess.State != StateTypingReply || sess.PendingChoice != "most impressive skill" {
		t.Fatalf("expected to wait for value for 'most impressive skill'")
	}

	update.Message.Text = "Golang"
	handleReceivedInformation(b, update, sess, update.Message.Text)
	if got := sess.Data["most impressive skill"]; got != "golang" {
		t.Fatalf("expected value saved in lowercase, got %q", got)
	}
}

func TestDoneKeepsDataAndRemovesKeyboard(t *testing.T) {
	sess := &UserSession{
		State:         StateTypingReply,
		Data:          map[string]string{"age": "25"},
		PendingChoice: "",
	}
	b := &fakeBot{}
	update := tgbotapi.Update{Message: &tgbotapi.Message{
		Text: "Done",
		Chat: &tgbotapi.Chat{ID: 1},
		From: &tgbotapi.User{ID: 99},
	}}
	handleDone(b, update, sess)
	if sess.State != StateChoosing {
		t.Fatalf("expected state to return to choosing")
	}
	if sess.Data["age"] != "25" {
		t.Fatalf("data should be preserved")
	}
}
