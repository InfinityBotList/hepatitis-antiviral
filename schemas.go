package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hepatitis-antiviral/cli"
	"hepatitis-antiviral/migrations"
	"hepatitis-antiviral/sources/jsonfile"
	"hepatitis-antiviral/sources/mongo"
	"hepatitis-antiviral/transform"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/infinitybotlist/eureka/crypto"
	"go.mongodb.org/mongo-driver/bson"
)

var sess *discordgo.Session

var ctx = context.Background()

// Schemas here
//
// Either use schema struct tag (or bson + mark struct tag for special type overrides)

// Tool below

var source mongo.MongoSource

type UUID = string

type Bot struct {
	BotID            string        `src:"botID" dest:"bot_id" unique:"true"`
	QueueName        string        `src:"botName" dest:"queue_name"`
	QueueAvatar      string        `src:"avatar" dest:"queue_avatar"`
	ClientID         string        `src:"clientID" dest:"client_id" unique:"true"`
	Tags             []string      `src:"tags" dest:"tags"`
	Prefix           *string       `src:"prefix" dest:"prefix"`
	Owner            string        `src:"main_owner" dest:"owner" fkey:"users,user_id"`
	AdditionalOwners []string      `src:"additional_owners" dest:"additional_owners" notnull:"true"`
	StaffBot         bool          `src:"staff" dest:"staff_bot" default:"false"`
	Short            string        `src:"short" dest:"short"`
	Long             string        `src:"long" dest:"long"`
	Library          *string       `src:"library" dest:"library" default:"'custom'"`
	ExtraLinks       []any         `src:"extra_links" dest:"extra_links" mark:"jsonb"`
	NSFW             bool          `src:"nsfw" dest:"nsfw" default:"false"`
	Premium          bool          `src:"premium" dest:"premium" default:"false"`
	PendingCert      bool          `src:"pending_cert" dest:"pending_cert" default:"false"`
	Servers          int           `src:"servers" dest:"servers" default:"0"`
	Shards           int           `src:"shards" dest:"shards" default:"0"`
	Users            int           `src:"users" dest:"users" default:"0"`
	ShardSet         []int         `src:"shardArray" dest:"shard_list" default:"{}"`
	Votes            int           `src:"votes" dest:"votes" default:"0"`
	Clicks           int           `src:"clicks" dest:"clicks" default:"0"`
	InviteClicks     int           `src:"invite_clicks" dest:"invite_clicks" default:"0"`
	Banner           *string       `src:"background,omitempty" dest:"banner" default:"null"`
	Invite           *string       `src:"invite" dest:"invite" default:"null"`
	Type             string        `src:"type" dest:"type" default:"'pending'"`
	Vanity           *string       `src:"vanity" dest:"vanity" unique:"true"`
	ExternalSource   string        `src:"external_source,omitempty" dest:"external_source" default:"null"`
	ListSource       string        `src:"listSource,omitempty" dest:"list_source" mark:"uuid" default:"null"`
	VoteBanned       bool          `src:"vote_banned,omitempty" dest:"vote_banned" default:"false" notnull:"true"`
	CrossAdd         bool          `src:"cross_add" dest:"cross_add" default:"true" notnull:"true"`
	StartPeriod      time.Time     `src:"start_period,omitempty" dest:"start_premium_period" default:"NOW()" notnull:"true"`
	SubPeriod        time.Duration `src:"sub_period,omitempty" dest:"premium_period_length" default:"'12 hours'" mark:"interval" notnull:"true"`
	CertReason       string        `src:"cert_reason,omitempty" dest:"cert_reason" default:"null"`
	Announce         bool          `src:"announce,omitempty" dest:"announce" default:"false"`
	AnnounceMessage  string        `src:"announce_msg,omitempty" dest:"announce_message" default:"null"`
	Uptime           int64         `src:"uptime,omitempty" dest:"uptime" default:"0"`
	TotalUptime      int64         `src:"total_uptime,omitempty" dest:"total_uptime" default:"0"`
	ClaimedBy        string        `src:"claimedBy,omitempty" dest:"claimed_by" default:"null"`
	Note             string        `src:"note,omitempty" dest:"approval_note" default:"'No note'" notnull:"true"`
	QueueReason      string        `src:"queue_reason,omitempty" dest:"queue_reason" default:"null"` // Reason bot was approved or denied
	Date             time.Time     `src:"date,omitempty" dest:"created_at" default:"NOW()" notnull:"true"`
	WebAuth          *string       `src:"webAuth,omitempty" dest:"web_auth" default:"null"`
	WebURL           *string       `src:"webURL,omitempty" dest:"webhook" default:"null"`
	WebHMac          *bool         `src:"webHMac" dest:"hmac" default:"false"`
	UniqueClicks     []string      `src:"unique_clicks,omitempty" dest:"unique_clicks" default:"{}" notnull:"true"`
	Token            string        `src:"token" dest:"api_token" default:"uuid_generate_v4()"`
	LastClaimed      time.Time     `src:"last_claimed,omitempty" dest:"last_claimed" default:"null"`
}

var botTransforms = map[string]cli.TransformFunc{
	"ClaimedBy": func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			return nil
		}

		if strings.ToLower(tr.CurrentValue.(string)) == "none" || tr.CurrentValue.(string) == "" {
			return nil
		}

		return tr.CurrentValue
	},
	"StartPeriod": func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			return nil
		}

		switch val := tr.CurrentValue.(type) {
		case time.Time:
			return val
		case int32:
			return time.Unix(0, int64(val*int32(time.Millisecond)))
		case int64:
			return time.Unix(0, int64(val*int64(time.Millisecond)))
		case float64:
			return time.Unix(0, int64(val*float64(time.Millisecond)))
		}

		panic("invalid type for StartPeriod")
	},
	"SubPeriod": func(tr cli.TransformRow) any {
		// Convert to time.Duration
		if tr.CurrentValue == nil {
			return nil
		}

		switch val := tr.CurrentValue.(type) {
		case time.Duration:
			return val
		case int32:
			return time.Duration(val) * time.Millisecond
		case int64:
			return time.Duration(val) * time.Millisecond
		case float64:
			return time.Duration(val) * time.Millisecond
		}

		panic("invalid type for SubPeriod: " + reflect.TypeOf(tr.CurrentValue).String())
	},
	"Type": func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			return "approved"
		}

		if val, ok := tr.CurrentRecord["certified"].(bool); ok && val {
			return "certified"
		}

		if tr.CurrentValue == "approved" || tr.CurrentValue == "denied" {
			return tr.CurrentValue
		}

		if val, ok := tr.CurrentRecord["claimed"].(bool); ok && val {
			return "claimed"
		}

		return tr.CurrentValue
	},
	"UniqueClicks": func(tr cli.TransformRow) any {
		return []string{}
	},
	"ExtraLinks": func(tr cli.TransformRow) any {
		var links = []link{}

		for _, cols := range []string{"Website", "Support", "Github", "Donate"} {
			col := strings.ToLower(cols)
			if tr.CurrentRecord[col] != nil {
				value := parseLink(col, tr.CurrentRecord[col].(string))

				if value == "" {
					continue
				}

				links = append(links, link{
					Name:  cols,
					Value: value,
				})
			}
		}

		return links
	},
	"ClientID": transform.DefaultTransform(func(tr cli.TransformRow) any {
		botId := tr.CurrentRecord["botID"].(string)

		cli.NotifyMsg("info", "No client ID for bot "+botId+", finding one")
		_, rerr := sess.Request("GET", "https://discord.com/api/v10/applications/"+botId+"/rpc", nil)

		if rerr == nil {
			source.Conn.Database("infinity").Collection("bots").UpdateOne(context.Background(), bson.M{
				"botID": botId,
			}, bson.M{
				"$set": bson.M{
					"clientID": botId,
				},
			})

			return botId
		}

		for rerr != nil {
			clientId := cli.PromptServerChannel("What is the client ID for " + botId + "?")

			if clientId == "DEL" {
				source.Conn.Database("infinity").Collection("bots").DeleteOne(context.Background(), bson.M{"botID": botId})
				return "SKIP"
			}

			_, rerr = sess.Request("GET", "https://discord.com/api/v10/applications/"+clientId+"/rpc", nil)

			if rerr != nil {
				fmt.Println("Client ID fetch error:", rerr)
				continue
			}

			source.Conn.Database("infinity").Collection("bots").UpdateOne(context.Background(), bson.M{
				"botID": botId,
			}, bson.M{
				"$set": bson.M{
					"clientID": clientId,
				},
			})

			return clientId
		}

		return botId
	}),
	"AdditionalOwners": transform.ToList,
	"Tags":             transform.ToList,
	"Owner": func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			return nil
		}

		userId := tr.CurrentValue.(string)

		userId = strings.TrimSpace(userId)

		var count int64

		err := cli.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE user_id = $1", userId).Scan(&count)

		if err != nil {
			panic(err)
		}

		if count == 0 {
			cli.NotifyMsg("warning", "User not found, adding")

			if _, err = cli.Pool.Exec(ctx, "INSERT INTO users (user_id, api_token, extra_links) VALUES ($1, $2, $3)", userId, crypto.RandString(128), []link{}); err != nil {
				panic(err)
			}
		}

		return userId
	},
	"Vanity": func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			cli.NotifyMsg("error", "Got nil name for current context: "+fmt.Sprint(tr.CurrentRecord["botID"]))
			panic("Got nil name")
		}

		name := tr.CurrentValue.(string)

		// Strip out unicode characters
		name = strings.Map(func(r rune) rune {
			if r > unicode.MaxASCII {
				return -1
			}
			return r
		}, name)

		if name == "" {
			panic("Got empty name")
		}

		name = strings.TrimSuffix(name, "-")

		// Check if vanity is taken
		var count int64

		err := cli.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM bots WHERE lower(vanity) = $1", strings.ToLower(name)).Scan(&count)

		if err != nil {
			panic(err)
		}

		if count != 0 {
			return strings.ToLower(name) + "-" + crypto.RandString(12)
		}

		return strings.ToLower(name)
	},
}

type ActionLog struct {
	BotID     string    `src:"botID" dest:"bot_id" fkey:"bots,bot_id"`
	StaffID   string    `src:"staff_id" dest:"staff_id" fkey:"users,user_id"`
	ActReason string    `src:"reason" dest:"action_reason" default:"'No reason'"`
	Timestamp time.Time `src:"ts" dest:"ts" default:"NOW()"`
	Event     string    `src:"event" dest:"event"`
}

type Claims struct {
	BotID       string    `src:"botID" dest:"bot_id" unique:"true" fkey:"bots,bot_id"`
	ClaimedBy   string    `src:"claimedBy" dest:"claimed_by"`
	Claimed     bool      `src:"claimed" dest:"claimed"`
	ClaimedAt   time.Time `src:"claimedAt" dest:"claimed_at" default:"NOW()"`
	UnclaimedAt time.Time `src:"unclaimedAt" dest:"unclaimed_at" default:"NOW()"`
}

type OnboardData struct {
	UserID      string         `src:"userID" dest:"user_id" fkey:"users,user_id"`
	OnboardCode string         `src:"onboard_code" dest:"onboard_code"`
	Data        map[string]any `src:"data" dest:"data" default:"{}"`
}

type User struct {
	UserID                    string    `src:"userID" dest:"user_id" unique:"true" default:"SKIP"`
	Username                  string    `src:"username" dest:"username" default:"User"`
	Experiments               []string  `src:"experiments" dest:"experiments" default:"{}"`
	StaffOnboarded            bool      `src:"staff_onboarded" dest:"staff_onboarded" default:"false"`
	StaffOnboardState         string    `src:"staff_onboard_state" dest:"staff_onboard_state" default:"'pending'"`
	StaffOnboardLastStartTime time.Time `src:"staff_onboard_last_start_time,omitempty" dest:"staff_onboard_last_start_time" default:"null"`
	StaffOnboardMacroTime     time.Time `src:"staff_onboard_macro_time,omitempty" dest:"staff_onboard_macro_time" default:"null"`
	StaffOnboardSessionCode   string    `src:"staff_onboard_session_code,omitempty" dest:"staff_onboard_session_code,omitempty" default:"null"`
	StaffOnboardGuild         string    `src:"staff_onboard_guild,omitempty" dest:"staff_onboard_guild,omitempty" default:"null"`
	Staff                     bool      `src:"staff" dest:"staff" default:"false"`
	Admin                     bool      `src:"admin" dest:"admin" default:"false"`
	HAdmin                    bool      `src:"hadmin" dest:"hadmin" default:"false"`
	Certified                 bool      `src:"certified" dest:"certified" default:"false"`
	IBLDev                    bool      `src:"ibldev" dest:"ibldev" default:"false"`
	IBLHDev                   bool      `src:"iblhdev" dest:"iblhdev" default:"false"`
	Developer                 bool      `src:"developer" dest:"developer" default:"false"`
	ExtraLinks                []any     `src:"extra_links" dest:"extra_links" mark:"jsonb"`
	APIToken                  string    `src:"apiToken" dest:"api_token"`
	About                     *string   `src:"about,omitempty" dest:"about" default:"'I am a very mysterious person'"`
	VoteBanned                bool      `src:"vote_banned,omitempty" dest:"vote_banned" default:"false"`
}

var userTransforms = map[string]cli.TransformFunc{
	"UserID": func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			return tr.CurrentValue
		}

		userId := tr.CurrentValue.(string)

		return strings.TrimSpace(userId)
	},
	"Username": transform.DefaultTransform(func(tr cli.TransformRow) any {
		userId := tr.CurrentRecord["userID"].(string)

		// Call http://localhost:8080/_duser/ID
		resp, err := http.Get("http://localhost:8080/_duser/" + userId)

		if err != nil {
			fmt.Println("User fetch error:", err)
			return nil
		}

		if resp.StatusCode != 200 {
			fmt.Println("User fetch error:", resp.StatusCode)
			return nil
		}

		// Read the response body
		body, err := io.ReadAll(resp.Body)

		if err != nil {
			fmt.Println("User fetch error:", err)
			return nil
		}

		var data struct {
			Username string `dest:"username"`
		}

		// Unmarshal the response body
		err = json.Unmarshal(body, &data)

		if err != nil {
			fmt.Println("User fetch error:", err)
			return nil
		}

		return data.Username
	}),
	"APIToken": transform.DefaultTransform(func(tr cli.TransformRow) any {
		return crypto.RandString(128)
	}),
	"ExtraLinks": func(tr cli.TransformRow) any {
		var links = []link{}

		for _, cols := range []string{"Website", "Github"} {
			col := strings.ToLower(cols)
			if tr.CurrentRecord[col] != nil {
				value := parseLink(col, tr.CurrentRecord[col].(string))

				if value == "" {
					continue
				}

				links = append(links, link{
					Name:  cols,
					Value: value,
				})
			}
		}

		return links
	},
}

type Announcements struct {
	UserID         string    `src:"userID" dest:"user_id" fkey:"users,user_id"`
	AnnouncementID string    `src:"announceID" dest:"id" mark:"uuid" default:"uuid_generate_v4()" omit:"true"`
	Title          string    `src:"title" dest:"title"`
	Content        string    `src:"content" dest:"content"`
	ModifiedDate   time.Time `src:"modifiedDate" dest:"modified_date" default:"NOW()"`
	ExpiresDate    time.Time `src:"expiresDate,omitempty" dest:"expires_date" default:"NOW()"`
	Status         string    `src:"status" dest:"status" default:"'active'"`
	Targetted      bool      `src:"targetted" dest:"targetted" default:"false"`
	Target         []string  `src:"target,omitempty" dest:"target" default:"null"`
}

var announcementTransforms = map[string]cli.TransformFunc{
	"AnnouncementID": transform.UUIDDefault,
}

type Votes struct {
	UserID string    `src:"userID" dest:"user_id" fkey:"users,user_id" fkignore:"true"`
	BotID  string    `src:"botID" dest:"bot_id" fkey:"bots,bot_id"`
	Date   time.Time `src:"date" dest:"created_at" default:"NOW()"`
}

type PackVotes struct {
	UserID string    `src:"userID" dest:"user_id" fkey:"users,user_id"`
	URL    string    `src:"url" dest:"url" fkey:"packs,url"`
	Upvote bool      `src:"upvote" dest:"upvote"`
	Date   time.Time `src:"date" dest:"created_at" default:"NOW()"`
}

type Packs struct {
	Owner string    `src:"owner" dest:"owner" fkey:"users,user_id"`
	Name  string    `src:"name" dest:"name" default:"'My pack'"`
	Short string    `src:"short" dest:"short"`
	Tags  []string  `src:"tags" dest:"tags"`
	URL   string    `src:"url" dest:"url" unique:"true"`
	Date  time.Time `src:"date" dest:"created_at" default:"NOW()"`
	Bots  []string  `src:"bots" dest:"bots"`
}

var packTransforms = map[string]cli.TransformFunc{
	"Tags": transform.ToList,
	"Bots": transform.ToList,
	"URL": func(tr cli.TransformRow) any {
		if tr.CurrentValue == nil {
			return crypto.RandString(12)
		}

		reg, _ := regexp.Compile("[^a-zA-Z0-9 ]+")

		return reg.ReplaceAllString(tr.CurrentValue.(string), "")
	},
}

type Reviews struct {
	ID       string    `src:"review_id" unique:"true" dest:"id" mark:"uuid" default:"uuid_generate_v4()" omit:"true"`
	BotID    string    `src:"botID" dest:"bot_id" fkey:"bots,bot_id"`
	Author   string    `src:"author" dest:"author" fkey:"users,user_id"`
	Content  string    `src:"content" dest:"content" default:"'Very good bot!'"`
	Rate     bool      `src:"rate" dest:"rate" default:"true"`
	StarRate int       `src:"star_rate" dest:"stars" default:"1"`
	Date     time.Time `src:"date" dest:"created_at" default:"NOW()"`
	ParentID string    `src:"parentID,omitempty" dest:"parent_id" default:"null"`
}

var reviewTransforms = map[string]cli.TransformFunc{
	"ID": transform.UUIDDefault,
}

type Tickets struct {
	ChannelID     string    `src:"channelID" dest:"channel_id"`
	TopicID       string    `src:"topicID" dest:"topic_id"`
	Topic         string    `src:"topic" dest:"topic" mark:"jsonb" default:"'{}'"`
	Issue         string    `src:"issue" dest:"issue"`
	TicketContext string    `src:"ticketContext" dest:"ticket_context" mark:"jsonb" default:"'{}'"`
	Messages      string    `src:"messages" dest:"messages" mark:"jsonb" default:"'{}'"`
	UserID        string    `src:"userID" dest:"user_id"` // No fkey here bc a user may not be a user on the table yet
	TicketID      string    `src:"ticketID" dest:"id" unique:"true"`
	CloseUserID   string    `src:"closeUserID,omitempty" dest:"close_user_id" default:"null"`
	Open          bool      `src:"open" dest:"open" default:"true"`
	Date          time.Time `src:"date" dest:"created_at" default:"NOW()"`
}

type Alerts struct {
	UserID  string `src:"userID" dest:"user_id" fkey:"users,user_id"`
	URL     string `src:"url" dest:"url"`
	Message string `src:"message" dest:"message"`
	Type    string `src:"type" dest:"type"`
}

type Poppypaw struct {
	UserID    string    `src:"id" dest:"user_id" fkey:"users,user_id"`
	NotifID   string    `src:"notifId" dest:"notif_id"`
	Auth      string    `src:"auth" dest:"auth"`
	P256dh    string    `src:"p256dh" dest:"p256dh"`
	Endpoint  string    `src:"endpoint" dest:"endpoint"`
	CreatedAt time.Time `src:"createdAt" dest:"created_at" default:"NOW()"`
	UA        string    `src:"ua" dest:"ua" default:"''"`
}

type Silverpelt struct {
	UserID    string    `src:"userID" dest:"user_id" fkey:"users,user_id"`
	BotID     string    `src:"botID" dest:"bot_id" fkey:"bots,bot_id"`
	CreatedAt time.Time `src:"createdAt" dest:"created_at" default:"NOW()"`
	LastAcked time.Time `src:"lastAcked" dest:"last_acked" default:"NOW()"`
}

type Apps struct {
	AppID            string         `src:"appID" dest:"app_id"`
	UserID           string         `src:"userID" dest:"user_id" fkey:"users,user_id"`
	Position         string         `src:"position" dest:"position"`
	CreatedAt        time.Time      `src:"createdAt" dest:"created_at" default:"NOW()"`
	Answers          map[string]any `src:"answers" dest:"answers" default:"{}"`
	InterviewAnswers map[string]any `src:"interviewAnswers" dest:"interview_answers" default:"{}"`
	State            string         `src:"state" dest:"state" default:"'pending'"`
}

func main() {
	// Place all schemas to be used in the tool here

	cli.Main(cli.App{
		SchemaOpts: cli.SchemaOpts{
			TableName: "infinity",
		},
		// Required
		LoadSource: func(name string) (cli.Source, error) {
			sess, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))

			if err != nil {
				panic(err)
			}

			sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

			err = sess.Open()

			if err != nil {
				panic(err)
			}

			switch name {
			case "mongo":
				source = mongo.MongoSource{
					ConnectionURL:  os.Getenv("MONGO"),
					DatabaseName:   "infinity",
					IgnoreEntities: []string{"sessions"},
				}

				err := source.Connect()

				if err != nil {
					return nil, err
				}

				return source, nil
			case "json":
				jsonSource := jsonfile.JsonFileStore{
					Filename:       "backup.json",
					IgnoreEntities: []string{"sessions"},
				}

				err := jsonSource.Connect()

				if err != nil {
					return nil, err
				}

				return jsonSource, nil
			}

			return nil, errors.New("unknown source")
		},
		// Optional, experimental
		BackupFunc: func(source cli.Source) {
			var err error
			sess, err = discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))

			if err != nil {
				panic(err)
			}

			sess.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

			err = sess.Open()

			if err != nil {
				panic(err)
			}

			cli.BackupTool(source, "users", User{}, cli.BackupOpts{
				IgnoreFKError:     true,
				IgnoreUniqueError: true,
				Transforms:        userTransforms,
			})

			cli.BackupTool(source, "apps", Apps{}, cli.BackupOpts{})

			cli.BackupTool(source, "bots", Bot{}, cli.BackupOpts{
				IndexCols:  []string{"bot_id", "staff_bot", "cross_add", "api_token", "lower(vanity)"},
				Transforms: botTransforms,
			})
			cli.BackupTool(source, "claims", Claims{}, cli.BackupOpts{
				RenameTo: "reports",
			})
			cli.BackupTool(source, "announcements", Announcements{}, cli.BackupOpts{
				Transforms: announcementTransforms,
			})
			cli.BackupTool(source, "votes", Votes{}, cli.BackupOpts{
				IgnoreFKError: true,
			})
			cli.BackupTool(source, "packages", Packs{}, cli.BackupOpts{
				IgnoreFKError: true,
				RenameTo:      "packs",
				Transforms:    packTransforms,
			})
			cli.BackupTool(source, "reviews", Reviews{}, cli.BackupOpts{
				IgnoreFKError: true,
				Transforms:    reviewTransforms,
			})
			cli.BackupTool(source, "tickets2", Tickets{}, cli.BackupOpts{
				IgnoreFKError: true,
				RenameTo:      "tickets",
			})

			cli.BackupTool(source, "poppypaw", Poppypaw{}, cli.BackupOpts{})

			cli.BackupTool(source, "silverpelt", Silverpelt{}, cli.BackupOpts{})

			cli.BackupTool(source, "alerts", Alerts{}, cli.BackupOpts{})

			cli.BackupTool(source, "action_logs", ActionLog{}, cli.BackupOpts{})

			cli.BackupTool(source, "onboard_data", OnboardData{}, cli.BackupOpts{})

			cli.BackupTool(source, "pack_votes", PackVotes{}, cli.BackupOpts{})

			migrations.Migrate(context.Background(), cli.Pool)

			cli.Pool.Exec(context.Background(), "DELETE FROM bots WHERE bot_id = 'SKIP' OR client_id = 'SKIP'")
		},
	})
}
