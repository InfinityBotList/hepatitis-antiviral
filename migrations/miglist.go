// Put your migration functions here
package migrations

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"hepatitis-antiviral/cli"
	"strings"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slices"
)

type link struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

var miglist = []migrator{
	{
		name: "add_extra_links bots",
		fn: func(ctx context.Context, pool *pgxpool.Pool) {
			_, err := pool.Exec(ctx, "ALTER TABLE bots ADD COLUMN extra_links jsonb NOT NULL DEFAULT '{}'")
			if err != nil {
				panic(err)
			}

			// get every website, support, donate and github link
			rows, err := pool.Query(ctx, "SELECT bot_id, website, support, github, donate FROM bots")

			if err != nil {
				panic(err)
			}

			// get count of rows
			var count int64

			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM bots").Scan(&count)

			if err != nil {
				panic(err)
			}

			migBar := cli.StartBar("bot migration", count, false)

			defer rows.Close()

			for rows.Next() {
				migBar.Increment()

				var botID pgtype.Text
				var website, support, github, donate pgtype.Text

				err = rows.Scan(&botID, &website, &support, &github, &donate)

				if err != nil {
					panic(err)
				}

				var cols = []link{}

				if !isNone(website) {
					cols = append(cols, link{
						Name:  "Website",
						Value: website.String,
					})
				}

				if !isNone(support) {
					cols = append(cols, link{
						Name:  "Support",
						Value: support.String,
					})
				}

				if !isNone(github) {
					cols = append(cols, link{
						Name:  "GitHub",
						Value: github.String,
					})
				}

				if !isNone(donate) {
					cols = append(cols, link{
						Name:  "Donate",
						Value: donate.String,
					})
				}

				_, err = pool.Exec(ctx, "UPDATE bots SET extra_links = $1 WHERE bot_id = $2", cols, botID.String)

				if err != nil {
					panic(err)
				}
			}

			_, err = pool.Exec(ctx, "ALTER TABLE bots DROP COLUMN support, DROP COLUMN github, DROP COLUMN donate, DROP COLUMN website")

			if err != nil {
				panic(err)
			}

			XSSCheck(ctx, pool)
		},
	},
	{
		name: "add_extra_links users",
		fn: func(ctx context.Context, pool *pgxpool.Pool) {
			_, err := pool.Exec(ctx, "ALTER TABLE users ADD COLUMN extra_links jsonb NOT NULL DEFAULT '{}'")
			if err != nil {
				panic(err)
			}

			// get every website and github link
			rows, err := pool.Query(ctx, "SELECT user_id, website, github FROM users")

			if err != nil {
				panic(err)
			}

			// get count of rows
			var count int64

			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)

			if err != nil {
				panic(err)
			}

			migBar := cli.StartBar("user migration", count, false)

			defer rows.Close()

			for rows.Next() {
				migBar.Increment()

				var userID pgtype.Text
				var website, github pgtype.Text

				err = rows.Scan(&userID, &website, &github)

				if err != nil {
					panic(err)
				}

				var cols = []link{}

				if !isNone(website) {
					cols = append(cols, link{
						Name:  "Website",
						Value: website.String,
					})
				}

				if !isNone(github) {
					cols = append(cols, link{
						Name:  "GitHub",
						Value: github.String,
					})
				}

				_, err = pool.Exec(ctx, "UPDATE users SET extra_links = $1 WHERE user_id = $2", cols, userID.String)

				if err != nil {
					panic(err)
				}
			}

			_, err = pool.Exec(ctx, "ALTER TABLE users DROP COLUMN github, DROP COLUMN website")

			if err != nil {
				panic(err)
			}

			XSSCheckUser(ctx, pool)
		},
	},
	{
		name: "hash unique_clicks (privacy)",
		fn: func(ctx context.Context, pool *pgxpool.Pool) {
			// get every website and github link
			rows, err := pool.Query(ctx, "SELECT bot_id, unique_clicks FROM bots")

			if err != nil {
				panic(err)
			}

			// get count of rows
			var count int64

			err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM bots").Scan(&count)

			if err != nil {
				panic(err)
			}

			migBar := cli.StartBar("hash unique_clicks (privacy)", count, false)

			defer rows.Close()

			for rows.Next() {
				migBar.Increment()

				var botID pgtype.Text
				var uniqueClicks []string

				err = rows.Scan(&botID, &uniqueClicks)

				if err != nil {
					panic(err)
				}

				var uniqueClicksHashed []string = make([]string, len(uniqueClicks))

				for _, v := range uniqueClicks {
					sha_512 := sha512.New()

					sha_512.Write([]byte(v))

					sum := sha_512.Sum(nil)

					hexedSum := hex.EncodeToString(sum)

					uniqueClicksHashed = append(uniqueClicksHashed, hexedSum)
				}

				_, err = pool.Exec(ctx, "UPDATE bots SET unique_clicks = $1 WHERE bot_id = $2", uniqueClicksHashed, botID.String)

				if err != nil {
					panic(err)
				}
			}
		},
	},
}

// XSS Checking functions

func parseLink(key string, link string) string {
	if strings.HasPrefix(link, "http://") {
		return strings.Replace(link, "http://", "https://", 1)
	}

	if strings.HasPrefix(link, "https://") {
		return link
	}

	cli.NotifyMsg("info", "Possibly Invalid URL found: "+link)

	if key == "Support" && !strings.Contains(link, " ") {
		link = strings.Replace(link, "www", "", 1)
		if strings.HasPrefix(link, "discord.gg/") {
			link = "https://discord.gg/" + link[11:]
		} else if strings.HasPrefix(link, "discord.com/invite/") {
			link = "https://discord.gg/" + link[19:]
		} else if strings.HasPrefix(link, "discord.com/") {
			link = "https://discord.gg/" + link[12:]
		} else {
			link = "https://discord.gg/" + link
		}
		cli.NotifyMsg("info", "Succesfully fixed support link to"+link)
		return link
	} else {
		// But wait, it may be safe still
		split := strings.Split(link, "/")[0]
		tldLst := strings.Split(split, ".")

		if len(tldLst) > 1 && (len(tldLst[len(tldLst)-1]) == 2 || slices.Contains([]string{
			"com",
			"net",
			"org",
			"fun",
			"app",
			"dev",
			"xyz",
		}, tldLst[len(tldLst)-1])) {
			cli.NotifyMsg("info", "Fixed found URL link to https://"+link)
			return "https://" + link
		} else {
			if strings.HasPrefix(link, "https://") {
				return link
			}

			cli.NotifyMsg("warning", "Removing invalid link: "+link)
			return ""
		}
	}
}

func XSSCheck(ctx context.Context, pool *pgxpool.Pool) {
	// get count of rows
	var count int64

	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM bots").Scan(&count)

	if err != nil {
		panic(err)
	}

	xssBar := cli.StartBar("XSS Check (bots)", count, false)

	// get every extra_link
	rows, err := pool.Query(ctx, "SELECT bot_id, extra_links FROM bots")

	if err != nil {
		panic(err)
	}

	defer rows.Close()

	for rows.Next() {
		xssBar.Increment()

		var botID pgtype.Text

		var extraLinks pgtype.JSONB

		err = rows.Scan(&botID, &extraLinks)

		if err != nil {
			panic(err)
		}

		var links []link

		err = extraLinks.AssignTo(&links)

		if err != nil {
			panic(err)
		}

		var parsedLinks = []link{}

		for k := range links {
			if links[k].Value == "" {
				continue
			}

			links[k].Value = strings.Trim(links[k].Value, " ")

			// Internal links are not validated
			if strings.HasPrefix(links[k].Value, "_") {
				cli.NotifyMsg("debug", "Internal link found, skipping validation")
				continue
			}

			// Validate URL
			value := parseLink(links[k].Name, links[k].Value)

			cli.NotifyMsg("debug", "Parsed link for "+links[k].Name+" is "+value)

			parsedLinks = append(parsedLinks, link{
				Name:  links[k].Name,
				Value: value,
			})
		}

		_, err = pool.Exec(ctx, "UPDATE bots SET extra_links = $1 WHERE bot_id = $2", parsedLinks, botID.String)

		if err != nil {
			panic(err)
		}
	}
}

func XSSCheckUser(ctx context.Context, pool *pgxpool.Pool) {
	// get count of rows
	var count int64

	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)

	if err != nil {
		panic(err)
	}

	xssBar := cli.StartBar("XSS Check (users)", count, false)

	// get every extra_link
	rows, err := pool.Query(ctx, "SELECT user_id, extra_links FROM users")

	if err != nil {
		panic(err)
	}

	defer rows.Close()

	for rows.Next() {
		xssBar.Increment()

		var userID pgtype.Text

		var extraLinks pgtype.JSONB

		err = rows.Scan(&userID, &extraLinks)

		if err != nil {
			panic(err)
		}

		var links []link

		err = extraLinks.AssignTo(&links)

		if err != nil {
			panic(err)
		}

		var parsedLinks = []link{}

		for k := range links {
			if links[k].Value == "" {
				continue
			}

			links[k].Value = strings.Trim(links[k].Value, " ")

			// Internal links are not validated
			if strings.HasPrefix(links[k].Value, "_") {
				cli.NotifyMsg("debug", "Internal link found, skipping validation")
				continue
			}

			// Validate URL
			value := parseLink(links[k].Name, links[k].Value)

			cli.NotifyMsg("debug", "Parsed link for "+links[k].Name+" is "+value)

			parsedLinks = append(parsedLinks, link{
				Name:  links[k].Name,
				Value: value,
			})
		}

		_, err = pool.Exec(ctx, "UPDATE users SET extra_links = $1 WHERE user_id = $2", parsedLinks, userID.String)

		if err != nil {
			panic(err)
		}
	}
}
