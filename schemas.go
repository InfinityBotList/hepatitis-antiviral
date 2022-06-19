package main

import "time"

// Schemas here
//
// Either use schema struct tag (or bson + mark struct tag for special type overrides)

// Tool below

type Auth struct {
	ID    *string `bson:"id,omitempty" schema:"id,text not null"`
	Token string  `bson:"token" schema:"token,text not null"`
}

type UUID = string

type Bot struct {
	BotID            string    `bson:"botID" json:"bot_id" mark:"text not null unique"`
	Name             string    `bson:"botName" json:"name"`
	TagsRaw          string    `bson:"tags" json:"tags" tolist:"true"`
	Prefix           *string   `bson:"prefix" json:"prefix"`
	Owner            string    `bson:"main_owner" json:"owner" log:"1"`
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
	Note             string    `bson:"note,omitempty" json:"approval_note" default:"No note"`
	Date             time.Time `bson:"date,omitempty" json:"date" mark:"timestamptz" schema:"date,timestamptz not null default NOW()"`
	Webhook          *string   `bson:"webhook,omitempty" json:"webhook" default:"null"` // Discord
	WebAuth          *string   `bson:"webAuth,omitempty" json:"web_auth" default:"null"`
	WebURL           *string   `bson:"webURL,omitempty" json:"custom_webhook" default:"null"`
	UniqueClicks     []string  `bson:"unique_clicks,omitempty" json:"unique_clicks" default:"{}"`
	Token            string    `bson:"token,omitempty" json:"token" default:"uuid_generate_v4()::text"`
}

// Place all schemas to be used in the tool here
func backupSchemas() {
	backupTool("oauths", Auth{})
	backupTool("bots", Bot{})
}
