package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Schemas here
//
// Either use schema struct tag (or bson + mark struct tag for special type overrides)

// Tool below

type Auth struct {
	ID    *string `bson:"id,omitempty" json:"id" notnull:"true"`
	Token string  `bson:"token" json:"token" notnull:"true"`
}

type UUID = string

type Bot struct {
	BotID            string    `bson:"botID" json:"bot_id" unique:"true"`
	Name             string    `bson:"botName" json:"name"`
	TagsRaw          string    `bson:"tags" json:"tags" tolist:"true"`
	Prefix           *string   `bson:"prefix" json:"prefix"`
	Owner            string    `bson:"main_owner" json:"owner"`
	AdditionalOwners []string  `bson:"additional_owners" json:"additional_owners"`
	StaffBot         bool      `bson:"staff" json:"staff_bot" default:"false"`
	Short            string    `bson:"short" json:"short"`
	Long             string    `bson:"long" json:"long"`
	Library          *string   `bson:"library,omitempty" json:"library" default:"null"`
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
	Vanity           *string   `bson:"vanity,omitempty" json:"vanity" default:"null"`
	ExternalSource   string    `bson:"external_source,omitempty" json:"external_source" default:"null"`
	ListSource       string    `bson:"listSource,omitempty" json:"list_source" mark:"uuid" default:"null"`
	VoteBanned       bool      `bson:"vote_banned,omitempty" json:"vote_banned" default:"false"`
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
	UniqueClicks     []string  `bson:"unique_clicks,omitempty" json:"unique_clicks" default:"{}"`
	Token            string    `bson:"token,omitempty" json:"token" default:"uuid_generate_v4()::text"`
}

type Claims struct {
	BotID       string    `bson:"botID" json:"bot_id" unique:"true" fkey:"bots,bot_id"`
	ClaimedBy   string    `bson:"claimedBy" json:"claimed_by"`
	Claimed     bool      `bson:"claimed" json:"claimed"`
	ClaimedAt   time.Time `bson:"claimedAt" json:"claimed_at" default:"NOW()"`
	UnclaimedAt time.Time `bson:"unclaimedAt" json:"unclaimed_at" default:"NOW()"`
}

type User struct {
	UserID        string         `bson:"userID" json:"user_id" unique:"true"`
	Username      string         `bson:"username" json:"username" defaultfunc:"getuser" default:"User"`
	Votes         map[string]any `bson:"votes" json:"votes" default:"{}"`
	PackVotes     map[string]any `bson:"pack_votes" json:"pack_votes" default:"{}"`
	Staff         bool           `bson:"staff" json:"staff" default:"false"`
	Admin         bool           `bson:"admin" json:"admin" default:"false"`
	Certified     bool           `bson:"certified" json:"certified" default:"false"`
	Developer     bool           `bson:"developer" json:"developer" default:"false"`
	Notifications bool           `bson:"notifications" json:"notifications" default:"false"`
	Website       *string        `bson:"website,omitempty" json:"website" default:"null"`
	Github        *string        `bson:"github,omitempty" json:"github" default:"null"`
	Nickname      *string        `bson:"nickname,omitempty" json:"nickname" default:"null"`
	APIToken      string         `bson:"apiToken" json:"api_token" default:"uuid_generate_v4()::text"`
	About         *string        `bson:"about,omitempty" json:"about" default:"'I am a very mysterious person'"`
	VoteBanned    bool           `bson:"vote_banned,omitempty" json:"vote_banned" default:"false"`
	StaffStats    map[string]any `bson:"staff_stats" json:"staff_stats" default:"{}"`
	NewStaffStats map[string]any `bson:"new_staff_stats" json:"new_staff_stats" default:"{}"`
}

type Announcements struct {
	UserID         string    `bson:"userID" json:"user_id" fkey:"users,user_id"`
	AnnouncementID string    `bson:"announceID" json:"id" mark:"uuid" default:"uuid_generate_v4()" omit:"true"`
	Title          string    `bson:"title" json:"title"`
	Content        string    `bson:"content" json:"content"`
	ModifiedDate   time.Time `bson:"modifiedDate" json:"modified_date" default:"NOW()"`
	ExpiresDate    time.Time `bson:"expiresDate,omitempty" json:"expires_date" default:"NOW()"`
	Status         string    `bson:"status" json:"status" default:"'active'"`
	Targetted      bool      `bson:"targetted" json:"targetted" default:"false"`
	Target         []string  `bson:"target,omitempty" json:"target" default:"null"`
}

// Exported functions
var exportedFuncs = map[string]*gfunc{
	"getuser": {
		param: "userID", // The parameter from mongo to accept
		function: func(p any) any {
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

			// Update mongodb with the username
			client.Database("infinity").Collection("users").UpdateOne(ctx, bson.M{"userID": userId}, bson.M{"$set": bson.M{"username": data.Username}})

			return data.Username
		},
	},
}

// Place all schemas to be used in the tool here
func backupSchemas() {
	backupTool("oauths", Auth{}, backupOpts{
		Concurrent: false,
	})
	backupTool("bots", Bot{}, backupOpts{
		Concurrent: false,
	})
	backupTool("claims", Claims{}, backupOpts{
		Concurrent: false,
	})
	backupTool("users", User{}, backupOpts{
		Concurrent: false,
	})
	backupTool("announcements", Announcements{}, backupOpts{
		Concurrent: false,
	})
}
