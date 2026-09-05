package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	tele "gopkg.in/telebot.v3"
)

type ChatUser struct {
	ChatID   int64 `gorm:"primaryKey"`
	UserID   int64 `gorm:"primaryKey"`
	Username string
}

type SentMessage struct {
	gorm.Model
	MessageId int
	ChatID    int64
}

type ChatStats struct {
	ChatID       int64 `gorm:"primaryKey"`
	LastActiveAt time.Time
	UsersCount   int64
}

var DB *gorm.DB

func ConnectDB() {
	var err error

	err = os.MkdirAll(filepath.Join(".", "data"), os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	DB, err = gorm.Open(sqlite.Open(filepath.Join(".", "data", "db.sqlite3")), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	err = DB.AutoMigrate(&ChatUser{})
	if err != nil {
		log.Fatal(err)
	}

	err = DB.AutoMigrate(&SentMessage{})
	if err != nil {
		log.Fatal(err)
	}

	err = DB.AutoMigrate(&ChatStats{})
	if err != nil {
		log.Fatal(err)
	}
}

func InitBot() *tele.Bot {
	client := &http.Client{Timeout: time.Minute}

	proxyURL := os.Getenv("HTTPS_PROXY")
	if proxyURL == "" {
		proxyURL = os.Getenv("HTTP_PROXY")
	}
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			log.Fatalf("failed to parse proxy URL: %v", err)
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxy),
		}
		log.Println("using proxy:", proxyURL)
	}

	b, err := tele.NewBot(tele.Settings{
		Token:  os.Getenv("TELEGRAM_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		Client: client,
	})
	if err != nil {
		log.Fatal(err)
	}
	return b
}

func logger(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		log.Println("user", c.Sender().ID, "sent", c.Message().Text)
		return next(c)
	}
}

func touchChatStats(c tele.Context) {
	DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_active_at"}),
	}).Create(&ChatStats{ChatID: c.Chat().ID, LastActiveAt: time.Now()})
}

func handleStart(c tele.Context) error {
	touchChatStats(c)
	return c.Send("Hey! I can help notify everyone 📢 in the group when someone needs them. " +
		"Everyone who wishes to receive mentions needs to /in to opt-in. " +
		"All opted-in users can then be mentioned using /all")
}

func handleIn(c tele.Context) error {
	touchChatStats(c)
	username := extractUsername(c.Sender())
	DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"username"}),
	}).Create(&ChatUser{ChatID: c.Message().Chat.ID, UserID: c.Sender().ID, Username: username})
	return c.Send("Thanks for opting in " + username)
}

func extractUsername(m *tele.User) string {
	if len(m.Username) > 0 {
		return m.Username
	}
	if len(m.FirstName) > 0 {
		return m.FirstName
	}
	return "anonymous"
}

func handleOut(c tele.Context) error {
	touchChatStats(c)
	DB.Where("chat_id = ? and user_id = ?", c.Chat().ID, c.Sender().ID).Delete(&ChatUser{})
	msg := fmt.Sprintf("You've been opted out %v", extractUsername(c.Sender()))
	return c.Send(msg)
}

func handleAll(c tele.Context) error {
	var users []ChatUser
	DB.Find(&users, ChatUser{ChatID: c.Chat().ID})

	DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"users_count", "last_active_at"}),
	}).Create(&ChatStats{ChatID: c.Chat().ID, LastActiveAt: time.Now(), UsersCount: int64(len(users))})

	if len(users) == 0 {
		return c.Send("There are no users. To opt in type /in command")
	}

	var mentions []string
	for _, chatUser := range users {
		croppedUsername := string([]rune(chatUser.Username)[:10])
		mentions = append(mentions, fmt.Sprintf(`<a href="tg://user?id=%v">%v</a>`,
			chatUser.UserID, html.EscapeString(croppedUsername)))
	}

	const chunkSize = 4
	for i := 0; i < len(mentions); i += chunkSize {
		end := i + chunkSize
		if end > len(mentions) {
			end = len(mentions)
		}

		msg, err := c.Bot().Send(c.Recipient(), strings.Join(mentions[i:end], " "), tele.ModeHTML)
		if err != nil {
			return err
		}
		DB.Create(&SentMessage{
			MessageId: msg.ID,
			ChatID:    msg.Chat.ID,
		})
	}
	return nil
}

func deleteOldSentMessages(b *tele.Bot) {
	for range time.Tick(time.Second * 10) {
		deleteBefore := time.Now().Add(-47 * time.Hour)
		var msg SentMessage
		result := DB.Where("created_at < ?", deleteBefore).Limit(1).Find(&msg)
		if result.RowsAffected != 1 {
			continue
		}

		err := b.Delete(tele.StoredMessage{
			MessageID: strconv.Itoa(msg.MessageId),
			ChatID:    msg.ChatID,
		})
		if err == nil {
			log.Printf("deleted message %v from chat %v", msg.MessageId, msg.ChatID)
		} else {
			log.Printf("failed to delete message %v from chat %v: %s", msg.MessageId, msg.ChatID, err)
		}
		DB.Delete(&msg)
	}
}

func handleStats(c tele.Context) error {
	var userCount int64
	var chatCount int64
	var groupCount int64

	DB.Model(&ChatUser{}).Distinct("user_id").Count(&userCount)
	DB.Model(&ChatUser{}).Distinct("chat_id").Count(&chatCount)
	DB.Model(&ChatUser{}).Select("count(*)").Group("chat_id").Having("count(*) > 1").Count(&groupCount)

	msg := fmt.Sprintf("`Users:  %6d\nChats:  %6d\nGroups: %6d`", userCount, chatCount, groupCount)
	return c.Send(msg, tele.ModeMarkdownV2)
}

func statsRecent(c tele.Context) error {
	type Result struct {
		Users  int64
		Chats  int64
		Groups int64
		B0     int64
		B1     int64
		B5     int64
		B10    int64
		B25    int64
		B50    int64
		B100   int64
		B250   int64
		B500   int64
		BMore  int64
	}

	var result Result
	DB.Raw(`SELECT
		COALESCE(SUM(users_count), 0) AS users,
		COUNT(*) AS chats,
		COALESCE(SUM(CASE WHEN users_count > 1 THEN 1 ELSE 0 END), 0) AS groups,
		COALESCE(SUM(CASE WHEN users_count = 0 THEN 1 ELSE 0 END), 0) AS b0,
		COALESCE(SUM(CASE WHEN users_count = 1 THEN 1 ELSE 0 END), 0) AS b1,
		COALESCE(SUM(CASE WHEN users_count BETWEEN 2 AND 5 THEN 1 ELSE 0 END), 0) AS b5,
		COALESCE(SUM(CASE WHEN users_count BETWEEN 6 AND 10 THEN 1 ELSE 0 END), 0) AS b10,
		COALESCE(SUM(CASE WHEN users_count BETWEEN 11 AND 25 THEN 1 ELSE 0 END), 0) AS b25,
		COALESCE(SUM(CASE WHEN users_count BETWEEN 26 AND 50 THEN 1 ELSE 0 END), 0) AS b50,
		COALESCE(SUM(CASE WHEN users_count BETWEEN 51 AND 100 THEN 1 ELSE 0 END), 0) AS b100,
		COALESCE(SUM(CASE WHEN users_count BETWEEN 101 AND 250 THEN 1 ELSE 0 END), 0) AS b250,
		COALESCE(SUM(CASE WHEN users_count BETWEEN 251 AND 500 THEN 1 ELSE 0 END), 0) AS b500,
		COALESCE(SUM(CASE WHEN users_count > 500 THEN 1 ELSE 0 END), 0) AS bmore
	FROM chat_stats`).Scan(&result)

	msg := fmt.Sprintf("```\n"+
		"Users: %d\n"+
		"Chats: %d\n"+
		"Groups: %d\n\n"+
		"Groups by size:\n"+
		"0:       %d\n"+
		"1:       %d\n"+
		"2-5:     %d\n"+
		"6-10: 	  %d\n"+
		"11-25:   %d\n"+
		"26-50:   %d\n"+
		"51-100:  %d\n"+
		"101-250: %d\n"+
		"251-500: %d\n"+
		"500+: 	  %d\n"+
		"```",
		result.Users, result.Chats, result.Groups,
		result.B0, result.B1, result.B5, result.B10, result.B25, result.B50,
		result.B100, result.B250, result.B500, result.BMore,
	)
	return c.Send(msg, tele.ModeMarkdownV2)
}

func handleUserLeft(c tele.Context) error {
	cu := ChatUser{UserID: c.Message().UserLeft.ID, ChatID: c.Chat().ID}
	log.Printf("user %d left chat %d", cu.UserID, cu.ChatID)
	DB.Where(&cu).Delete(&ChatUser{})
	return nil
}

func handleUserJoined(c tele.Context) error {
	u := c.Message().UserJoined
	if u.IsBot {
		return nil
	}
	cu := ChatUser{UserID: u.ID, ChatID: c.Chat().ID, Username: extractUsername(u)}
	log.Printf("user %d joined chat %d", cu.UserID, cu.ChatID)
	DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"username"}),
	}).Create(&cu)
	return nil
}

func handleCleanup(c tele.Context) error {
	log.Printf("started cleanup in chat %d", c.Chat().ID)
	users := make([]ChatUser, 0)
	deletedCount := 0
	DB.Where("chat_id = ?", c.Chat().ID).Find(&users)
	_ = c.Send("Started unsubscribing members who left the chat 🧹")
	for i, u := range users {
		time.Sleep(1 * time.Second)
		member, err := c.Bot().ChatMemberOf(c.Chat(), &tele.User{ID: u.UserID})
		if err != nil {
			if i == 0 {
				return c.Send("This command works only if the bot has admin privileges")
			} else {
				continue
			}
		}

		if member.Role == tele.Kicked || member.Role == tele.Left {
			DB.Where(&u).Delete(&ChatUser{})
			deletedCount++
			log.Printf("user %d is no longer member of chat %d", u.UserID, u.ChatID)
		}
	}
	return c.Send(fmt.Sprintf("Unsubscribed %d users", deletedCount))
}

func main() {
	ConnectDB()
	b := InitBot()
	b.Use(logger)
	b.Handle("/start", handleStart)
	b.Handle("/in", handleIn)
	b.Handle("/out", handleOut)
	b.Handle("/all", handleAll)
	b.Handle("/stats", handleStats)
	b.Handle("/stats_recent", statsRecent)
	b.Handle("/cleanup", handleCleanup)
	b.Handle(tele.OnUserLeft, handleUserLeft)
	b.Handle(tele.OnUserJoined, handleUserJoined)
	go deleteOldSentMessages(b)
	b.Start()
}
