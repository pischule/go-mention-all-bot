package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"os"
	"strings"
	"time"

	tb "gopkg.in/tucnak/telebot.v2"
)

type ChatUser struct {
	ChatID   int64 `gorm:"primaryKey"`
	UserID   int   `gorm:"primaryKey"`
	Username string
}

func main() {
	db, err := gorm.Open(sqlite.Open("bot_db.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	err = db.AutoMigrate(&ChatUser{})
	if err != nil {
		panic("failed to migrate")
	}

	b, err := tb.NewBot(tb.Settings{
		Token:  os.Getenv("TOKEN"),
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return
	}

	b.Handle("/start", func(m *tb.Message) {
		_, err := b.Send(
			m.Sender, "Hey! I can help notify everyone 📢 in the group when someone needs them.\n"+
				"Everyone who wishes to receive mentions needs to /in to opt-in. "+
				"All opted-in users can then be mentioned using /all",
		)
		if err != nil {
			log.Println(err)
			return
		}
	})

	b.Handle("/in", func(m *tb.Message) {
		user := m.Sender

		username := "anonymous"
		if len(user.Username) > 0 {
			username = user.Username
		} else if len(user.FirstName) > 0 {
			username = user.FirstName
		}

		db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chat_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"username"}),
		}).Create(&ChatUser{ChatID: m.Chat.ID, UserID: user.ID, Username: username})

		msg := fmt.Sprintf("Thanks for opting in %s", username)

		_, err := b.Send(m.Chat, msg)
		if err != nil {
			log.Println("/in", err)
			return
		}
	})

	b.Handle("/all", func(m *tb.Message) {
		var users []ChatUser
		db.Find(&users, ChatUser{ChatID: m.Chat.ID})

		var usersList []string

		for _, chatUser := range users {
			usersList = append(usersList, fmt.Sprintf("[yolo](tg://user?id=%v)", chatUser.UserID))
		}

		msg := strings.Join(usersList, " ")

		if len(users) == 0 {
			msg = "There are no users\\. To opt in type /in command"
		}

		_, err := b.Send(m.Chat, msg, tb.ModeMarkdownV2)

		if err != nil {
			log.Println("/all", err)
			return
		}
	})

	b.Handle("/out", func(m *tb.Message) {
		db.Where("chat_id = ? and user_id = ?", m.Chat.ID, m.Sender.ID).Delete(&ChatUser{})

		msg := fmt.Sprintf("You've been opted out %v", m.Sender.ID)

		_, err := b.Send(m.Chat, msg)
		if err != nil {
			log.Println("/out", err)
			return
		}
	})

	b.Handle("/stats", func(m *tb.Message) {
		var userCount int64
		var chatCount int64

		db.Model(&ChatUser{}).Distinct("user_id").Count(&userCount)
		db.Model(&ChatUser{}).Distinct("chat_id").Count(&chatCount)

		msg := fmt.Sprintf("Users: %5d\nChats: %5d", userCount, chatCount)

		_, err := b.Send(m.Chat, msg)
		if err != nil {
			log.Println(err)
			return
		}
	})

	b.Start()
}
