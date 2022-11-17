package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hepatitis-antiviral/cli"
	"hepatitis-antiviral/migrations"
	"hepatitis-antiviral/sources/mongo"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
)

var ctx = context.Background()

// Schemas here
//
// Either use schema struct tag (or bson + mark struct tag for special type overrides)

// Tool below

var source mongo.MongoSource

type Auth struct {
	ID    *string `bson:"id,omitempty" json:"id" notnull:"true"`
	Token string  `bson:"token" json:"token" notnull:"true"`
}

type UUID = string

type Bot struct {
	BotID            string    `bson:"botID" json:"bot_id" unique:"true"`
	ClientID         string    `bson:"clientID,omitempty" json:"client_id" default:"null"` // Its only nullable for now
	TagsRaw          string    `bson:"tags" json:"tags" tolist:"true"`
	Prefix           *string   `bson:"prefix" json:"prefix"`
	Owner            string    `bson:"main_owner" json:"owner" fkey:"users,user_id" pre:"usercheck"`
	AdditionalOwners []string  `bson:"additional_owners" json:"additional_owners"`
	StaffBot         bool      `bson:"staff" json:"staff_bot" default:"false"`
	Short            string    `bson:"short" json:"short"`
	Long             string    `bson:"long" json:"long"`
	Library          *string   `bson:"library" json:"library" default:"'custom'"`
	Website          *string   `bson:"website,omitempty" json:"website" default:"null"`
	Donate           *string   `bson:"donate,omitempty" json:"donate" default:"null"`
	Support          *string   `bson:"support,omitempty" json:"support" default:"null"`
	NSFW             bool      `bson:"nsfw" json:"nsfw" default:"false"`
	Premium          bool      `bson:"premium" json:"premium" default:"false"`
	Certified        bool      `bson:"certified" json:"certified" default:"false"`
	PendingCert      bool      `bson:"pending_cert" json:"pending_cert" default:"false"`
	Servers          int       `bson:"servers" json:"servers" default:"0"`
	Shards           int       `bson:"shards" json:"shards" default:"0"`
	Users            int       `bson:"users" json:"users" default:"0"`
	ShardSet         []int     `bson:"shardArray" json:"shard_list" default:"{}"`
	Votes            int       `bson:"votes" json:"votes" default:"0"`
	Clicks           int       `bson:"clicks" json:"clicks" default:"0"`
	InviteClicks     int       `bson:"invite_clicks" json:"invite_clicks" default:"0"`
	Github           *string   `bson:"github,omitempty" json:"github" default:"null"`
	Banner           *string   `bson:"background,omitempty" json:"banner" default:"null"`
	Invite           *string   `bson:"invite" json:"invite" default:"null"`
	Type             string    `bson:"type" json:"type"`
	Vanity           *string   `bson:"vanity" json:"vanity" pre:"updname" unique:"true"`
	ExternalSource   string    `bson:"external_source,omitempty" json:"external_source" default:"null"`
	ListSource       string    `bson:"listSource,omitempty" json:"list_source" mark:"uuid" default:"null"`
	VoteBanned       bool      `bson:"vote_banned,omitempty" json:"vote_banned" default:"false" notnull:"true"`
	CrossAdd         bool      `bson:"cross_add,omitempty" json:"cross_add" default:"true"`
	StartPeriod      int64     `bson:"start_period,omitempty" json:"start_premium_period" default:"0"`
	SubPeriod        int64     `bson:"sub_period,omitempty" json:"premium_period_length" default:"0"`
	CertReason       string    `bson:"cert_reason,omitempty" json:"cert_reason" default:"null"`
	Announce         bool      `bson:"announce,omitempty" json:"announce" default:"false"`
	AnnounceMessage  string    `bson:"announce_msg,omitempty" json:"announce_message" default:"null"`
	Uptime           int64     `bson:"uptime,omitempty" json:"uptime" default:"0"`
	TotalUptime      int64     `bson:"total_uptime,omitempty" json:"total_uptime" default:"0"`
	Claimed          bool      `bson:"claimed,omitempty" json:"claimed" default:"false"`
	ClaimedBy        string    `bson:"claimedBy,omitempty" json:"claimed_by" default:"null"`
	Note             string    `bson:"note,omitempty" json:"approval_note" default:"'No note'"`
	Date             time.Time `bson:"date,omitempty" json:"date" default:"NOW()" notnull:"true"`
	Webhook          *string   `bson:"webhook,omitempty" json:"webhook" default:"null"` // Discord
	WebAuth          *string   `bson:"webAuth,omitempty" json:"web_auth" default:"null"`
	WebURL           *string   `bson:"webURL,omitempty" json:"custom_webhook" default:"null"`
	WebHMac          *bool     `bson:"webHMac,omitempty" json:"hmac" default:"false"`
	UniqueClicks     []string  `bson:"unique_clicks,omitempty" json:"unique_clicks" default:"{}" notnull:"true"`
	Token            string    `bson:"token,omitempty" json:"token" default:"uuid_generate_v4()"`
	LastClaimed      time.Time `bson:"last_claimed,omitempty" json:"last_claimed" default:"null"`
}

type ActionLog struct {
	BotID     string    `bson:"botID" json:"bot_id" fkey:"bots,bot_id"`
	StaffID   string    `bson:"staff_id" json:"staff_id" fkey:"users,user_id"`
	ActReason string    `bson:"reason" json:"action_reason" default:"'No reason'"`
	Timestamp time.Time `bson:"ts" json:"ts" default:"NOW()"`
	Event     string    `bson:"event" json:"event"`
}

type Claims struct {
	BotID       string    `bson:"botID" json:"bot_id" unique:"true" fkey:"bots,bot_id"`
	ClaimedBy   string    `bson:"claimedBy" json:"claimed_by"`
	Claimed     bool      `bson:"claimed" json:"claimed"`
	ClaimedAt   time.Time `bson:"claimedAt" json:"claimed_at" default:"NOW()"`
	UnclaimedAt time.Time `bson:"unclaimedAt" json:"unclaimed_at" default:"NOW()"`
}

type OnboardData struct {
	UserID      string         `bson:"userID" json:"user_id" fkey:"users,user_id"`
	OnboardCode string         `bson:"onboard_code" json:"onboard_code"`
	Data        map[string]any `bson:"data" json:"data" default:"{}"`
}

type User struct {
	UserID                    string         `bson:"userID" json:"user_id" unique:"true" default:"SKIP" pre:"usertrim"`
	Username                  string         `bson:"username" json:"username" defaultfunc:"getuser" default:"User"`
	StaffOnboarded            bool           `bson:"staff_onboarded" json:"staff_onboarded" default:"false"`
	StaffOnboardState         string         `bson:"staff_onboard_state" json:"staff_onboard_state" default:"'pending'"`
	StaffOnboardLastStartTime time.Time      `bson:"staff_onboard_last_start_time,omitempty" json:"staff_onboard_last_start_time" default:"null"`
	StaffOnboardMacroTime     time.Time      `bson:"staff_onboard_macro_time,omitempty" json:"staff_onboard_macro_time" default:"null"`
	StaffOnboardSessionCode   string         `bson:"staff_onboard_session_code,omitempty" json:"staff_onboard_session_code,omitempty" default:"null"`
	Staff                     bool           `bson:"staff" json:"staff" default:"false"`
	Admin                     bool           `bson:"admin" json:"admin" default:"false"`
	HAdmin                    bool           `bson:"hadmin" json:"hadmin" default:"false"`
	Certified                 bool           `bson:"certified" json:"certified" default:"false"`
	IBLDev                    bool           `bson:"ibldev" json:"ibldev" default:"false"`
	IBLHDev                   bool           `bson:"iblhdev" json:"iblhdev" default:"false"`
	Developer                 bool           `bson:"developer" json:"developer" default:"false"`
	Notifications             bool           `bson:"notifications" json:"notifications" default:"false"`
	Website                   *string        `bson:"website,omitempty" json:"website" default:"null"`
	Github                    *string        `bson:"github,omitempty" json:"github" default:"null"`
	Nickname                  *string        `bson:"nickname,omitempty" json:"nickname" default:"null"`
	APIToken                  string         `bson:"apiToken" json:"api_token" defaultfunc:"gentoken"`
	About                     *string        `bson:"about,omitempty" json:"about" default:"'I am a very mysterious person'"`
	VoteBanned                bool           `bson:"vote_banned,omitempty" json:"vote_banned" default:"false"`
	StaffStats                map[string]any `bson:"staff_stats" json:"staff_stats" default:"{}"`
	NewStaffStats             map[string]any `bson:"new_staff_stats" json:"new_staff_stats" default:"{}"`
}

type Announcements struct {
	UserID         string    `bson:"userID" json:"user_id" fkey:"users,user_id"`
	AnnouncementID string    `bson:"announceID" json:"id" mark:"uuid" defaultfunc:"uuidgen" default:"uuid_generate_v4()" omit:"true"`
	Title          string    `bson:"title" json:"title"`
	Content        string    `bson:"content" json:"content"`
	ModifiedDate   time.Time `bson:"modifiedDate" json:"modified_date" default:"NOW()"`
	ExpiresDate    time.Time `bson:"expiresDate,omitempty" json:"expires_date" default:"NOW()"`
	Status         string    `bson:"status" json:"status" default:"'active'"`
	Targetted      bool      `bson:"targetted" json:"targetted" default:"false"`
	Target         []string  `bson:"target,omitempty" json:"target" default:"null"`
}

type Votes struct {
	UserID string    `bson:"userID" json:"user_id" fkey:"users,user_id" fkignore:"true"`
	BotID  string    `bson:"botID" json:"bot_id" fkey:"bots,bot_id"`
	Date   time.Time `bson:"date" json:"date" default:"NOW()"`
}

type PackVotes struct {
	UserID string    `bson:"userID" json:"user_id" fkey:"users,user_id"`
	URL    string    `bson:"url" json:"url" fkey:"packs,url"`
	Upvote bool      `bson:"upvote" json:"upvote"`
	Date   time.Time `bson:"date" json:"date" default:"NOW()"`
}

type Packs struct {
	Owner   string    `bson:"owner" json:"owner" fkey:"users,user_id"`
	Name    string    `bson:"name" json:"name" default:"'My pack'"`
	Short   string    `bson:"short" json:"short"`
	TagsRaw string    `bson:"tags" json:"tags" tolist:"true"`
	URL     string    `bson:"url" json:"url" unique:"true"`
	Date    time.Time `bson:"date" json:"date" default:"NOW()"`
	Bots    []string  `bson:"bots" json:"bots" tolist:"true"`
}

type Reviews struct {
	BotID       string         `bson:"botID" json:"bot_id" fkey:"bots,bot_id"`
	Author      string         `bson:"author" json:"author" fkey:"users,user_id"`
	Content     string         `bson:"content" json:"content" default:"'Very good bot!'"`
	Rate        bool           `bson:"rate" json:"rate" default:"true"`
	StarRate    int            `bson:"star_rate" json:"stars" default:"1"`
	LikesRaw    map[string]any `bson:"likes" json:"likes"`
	DislikesRaw map[string]any `bson:"dislikes" json:"dislikes"`
	Date        time.Time      `bson:"date" json:"date" default:"NOW()"`
	Replies     map[string]any `bson:"replies" json:"replies" default:"{}"`
	Editted     bool           `bson:"editted" json:"editted" default:"false"`
	Flagged     bool           `bson:"flagged" json:"flagged" default:"false"`
}

type Tickets struct {
	ChannelID      string    `bson:"channelID" json:"channel_id"`
	Topic          string    `bson:"topic" json:"topic" default:"'Support'"`
	UserID         string    `bson:"userID" json:"user_id"` // No fkey here bc a user may not be a user on the table yet
	TicketID       int       `bson:"ticketID" json:"id" unique:"true"`
	LogURL         string    `bson:"logURL,omitempty" json:"log_url" default:"null"`
	CloseUserID    string    `bson:"closeUserID,omitempty" json:"close_user_id" default:"null"`
	Open           bool      `bson:"open" json:"open" default:"true"`
	Date           time.Time `bson:"date" json:"date" default:"NOW()"`
	PanelMessageID string    `bson:"panelMessageID,omitempty" json:"panel_message_id" default:"null"`
	PanelChannelID string    `bson:"panelChannelID,omitempty" json:"panel_channel_id" default:"null"`
}

type Transcripts struct {
	TicketID int            `bson:"ticketID" json:"id" fkey:"tickets,id"`
	Data     map[string]any `bson:"data" json:"data" default:"{}"`
	ClosedBy map[string]any `bson:"closedBy" json:"closed_by" default:"{}"`
	OpenedBy map[string]any `bson:"openedBy" json:"opened_by" default:"{}"`
}

type Notifications struct {
	UserID  string `bson:"userID" json:"user_id" fkey:"users,user_id"`
	URL     string `bson:"url" json:"url"`
	Message string `bson:"message" json:"message"`
	Type    string `bson:"type" json:"type"`
}

type Poppypaw struct {
	UserID    string    `bson:"id" json:"id" fkey:"users,user_id"`
	NotifID   string    `bson:"notifId" json:"notif_id"`
	Auth      string    `bson:"auth" json:"auth"`
	P256dh    string    `bson:"p256dh" json:"p256dh"`
	Endpoint  string    `bson:"endpoint" json:"endpoint"`
	CreatedAt time.Time `bson:"createdAt" json:"created_at" default:"NOW()"`
	UA        string    `bson:"ua" json:"ua" default:"''"`
}

type Silverpelt struct {
	UserID    string    `bson:"userID" json:"user_id" fkey:"users,user_id"`
	BotID     string    `bson:"botID" json:"bot_id" fkey:"bots,bot_id"`
	CreatedAt time.Time `bson:"createdAt" json:"created_at" default:"NOW()"`
	LastAcked time.Time `bson:"lastAcked" json:"last_acked" default:"NOW()"`
}

type Apps struct {
	AppID            string         `bson:"appID" json:"app_id"`
	UserID           string         `bson:"userID" json:"user_id" fkey:"users,user_id"`
	Position         string         `bson:"position" json:"position"`
	CreatedAt        time.Time      `bson:"createdAt" json:"created_at" default:"NOW()"`
	Answers          map[string]any `bson:"answers" json:"answers" default:"{}"`
	InterviewAnswers map[string]any `bson:"interviewAnswers" json:"interview_answers" default:"{}"`
	State            string         `bson:"state" json:"state" default:"'pending'"`
	Likes            []int64        `bson:"likes" json:"likes" default:"{}"`
	Dislikes         []int64        `bson:"dislikes" json:"dislikes" default:"{}"`
}

// Exported functions

var exportedFuncs = map[string]*cli.ExportedFunction{
	"uuidgen": {
		Param: "userID",
		Function: func(p any) any {
			uuid := uuid.New()
			return uuid.String()
		},
	},
	"gentoken": {
		Param: "userID",
		Function: func(p any) any {
			return RandString(128)
		},
	},
	"usertrim": {
		Param: "userID",
		Function: func(p any) any {
			if p == nil {
				return p
			}

			userId := p.(string)

			return strings.TrimSpace(userId)
		},
	},
	// Checks if user exists, otherwise adds one
	"usercheck": {
		Param: "main_owner",
		Function: func(p any) any {
			if p == nil {
				return p
			}

			userId := p.(string)

			userId = strings.TrimSpace(userId)

			var count int64

			err := cli.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE user_id = $1", userId).Scan(&count)

			if err != nil {
				panic(err)
			}

			if count == 0 {
				cli.NotifyMsg("warning", "User not found, adding")

				if _, err = cli.Pool.Exec(ctx, "INSERT INTO users (user_id, api_token) VALUES ($1, $2)", p, RandString(128)); err != nil {
					panic(err)
				}
			}

			return userId
		},
	},
	"updname": {
		Param: "vanity",
		Function: func(p any) any {
			if p == nil {
				cli.NotifyMsg("info", "Got nil name, trying mongo")

				// Check if name is defined on mongo
				name := source.Conn.Database("infinity").Collection("bots").FindOne(ctx, bson.M{"botID": cli.Map["botID"]})

				if name.Err() != nil {
					panic(name.Err())
				}

				var bot bson.M

				name.Decode(&bot)

				if v, ok := bot["botName"]; ok {
					if v, ok := v.(string); ok {
						vanity := strings.ToLower(strings.ReplaceAll(v, " ", "-"))
						cli.NotifyMsg("info", "Got name from mongo: "+vanity)
						p = vanity
					}
				} else {
					return "unknown-" + RandString(12)
				}
			}

			name := p.(string)

			if name == "" {
				return "unknown-" + RandString(12)
			}

			// Check if vanity is taken
			var count int64

			err := cli.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM bots WHERE vanity = $1", strings.ToLower(name)).Scan(&count)

			if err != nil {
				panic(err)
			}

			if count != 0 {
				return name + "-" + RandString(12)
			}

			return name
		},
	},
	// Gets a user
	"getuser": {
		Param: "userID", // The parameter from mongo to accept
		Function: func(p any) any {
			userId := p.(string)

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
			body, err := ioutil.ReadAll(resp.Body)

			if err != nil {
				fmt.Println("User fetch error:", err)
				return nil
			}

			var data struct {
				Username string `json:"username"`
			}

			// Unmarshal the response body
			err = json.Unmarshal(body, &data)

			if err != nil {
				fmt.Println("User fetch error:", err)
				return nil
			}

			// Update mongodb with the username and avatar
			source.Conn.Database("infinity").Collection("users").UpdateOne(ctx, bson.M{"userID": userId}, bson.M{"$set": bson.M{"username": data.Username}})
			return data.Username
		},
	},
}

func main() {
	// Place all schemas to be used in the tool here

	cli.Main(cli.App{
		SchemaOpts: cli.SchemaOpts{
			TableName: "infinity",
		},
		BackupFunc: func() {
			source = mongo.MongoSource{
				ConnectionURL: os.Getenv("MONGO"),
				DatabaseName:  "infinity",
			}

			err := source.Connect()

			if err != nil {
				panic(err)
			}

			cli.BackupTool(source, "oauths", Auth{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})
			cli.BackupTool(source, "users", User{}, cli.BackupOpts{
				IgnoreFKError:     true,
				IgnoreUniqueError: true,
				ExportedFuncs:     exportedFuncs,
			})
			cli.BackupTool(source, "bots", Bot{}, cli.BackupOpts{
				IndexCols:     []string{"bot_id", "staff_bot", "cross_add", "token", "lower(vanity)"},
				ExportedFuncs: exportedFuncs,
			})
			cli.BackupTool(source, "claims", Claims{}, cli.BackupOpts{
				RenameTo:      "reports",
				ExportedFuncs: exportedFuncs,
			})
			cli.BackupTool(source, "announcements", Announcements{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})
			cli.BackupTool(source, "votes", Votes{}, cli.BackupOpts{
				IgnoreFKError: true,
				ExportedFuncs: exportedFuncs,
			})
			cli.BackupTool(source, "packages", Packs{}, cli.BackupOpts{
				IgnoreFKError: true,
				RenameTo:      "packs",
				ExportedFuncs: exportedFuncs,
			})
			cli.BackupTool(source, "reviews", Reviews{}, cli.BackupOpts{
				IgnoreFKError: true,
				ExportedFuncs: exportedFuncs,
			})
			cli.BackupTool(source, "tickets", Tickets{}, cli.BackupOpts{
				IgnoreFKError: true,
				ExportedFuncs: exportedFuncs,
			})

			cli.BackupTool(source, "transcripts", Transcripts{}, cli.BackupOpts{
				IgnoreFKError: true,
				ExportedFuncs: exportedFuncs,
			})

			cli.BackupTool(source, "poppypaw", Poppypaw{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})

			cli.BackupTool(source, "silverpelt", Silverpelt{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})

			cli.BackupTool(source, "notifications", Notifications{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})

			cli.BackupTool(source, "action_logs", ActionLog{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})

			cli.BackupTool(source, "onboard_data", OnboardData{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})

			cli.BackupTool(source, "apps", Apps{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})

			cli.BackupTool(source, "pack_votes", PackVotes{}, cli.BackupOpts{
				ExportedFuncs: exportedFuncs,
			})

			migrations.Migrate(context.Background(), cli.Pool)
		},
	})
}
