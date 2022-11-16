// Put your migration functions here
package migrations

import (
	"context"
	"fmt"
	"hepatitis-antiviral/cli"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/exp/slices"
)

var miglist = []migrator{
	{
		name: "add_extra_links",
		fn: func(ctx context.Context, pool *pgxpool.Pool) {
			if !tableExists(ctx, pool, "bots") {
				panic("required table bots does not exist")
			}

			if colExists(ctx, pool, "bots", "extra_links") && !colExists(ctx, pool, "bots", "support") {
				fmt.Println("Nothing to do")
				return
			}

			if colExists(ctx, pool, "bots", "extra_links") {
				_, err := pool.Exec(ctx, "ALTER TABLE bots DROP COLUMN extra_links")
				if err != nil {
					panic(err)
				}
			}

			_, err := pool.Exec(ctx, "ALTER TABLE bots ADD COLUMN extra_links jsonb NOT NULL DEFAULT '{}'")
			if err != nil {
				panic(err)
			}

			// get every website, support, donate and github link
			rows, err := pool.Query(ctx, "SELECT bot_id, website, support, github, donate FROM bots")

			if err != nil {
				panic(err)
			}

			defer rows.Close()

			for rows.Next() {
				var botID pgtype.Text
				var website, support, github, donate pgtype.Text

				err = rows.Scan(&botID, &website, &support, &github, &donate)

				if err != nil {
					panic(err)
				}

				var cols = make(map[string]string)

				if !isNone(website) {
					cols["Website"] = website.String
				}

				if !isNone(support) {
					cols["Support"] = support.String
				}

				if !isNone(github) {
					cols["Github"] = github.String
				}

				if !isNone(donate) {
					cols["Donate"] = donate.String
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
}

// XSS Checking functions

func parseLink(key string, link string) string {
	if strings.HasPrefix(link, "http://") {
		return strings.Replace(link, "http://", "https://", 1)
	}

	if strings.HasPrefix(link, "https://") {
		return link
	}

	fmt.Println("Invalid URL found:", link)

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
		fmt.Println("HOTFIX: Fixed support link to", link)
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
			fmt.Println("Fixed found URL link to", "https://"+link)
			return "https://" + link
		} else {
			if strings.HasPrefix(link, "https://") {
				return link
			}

			cli.NotifyMsg("warn", "Removing invalid link: "+link)
			time.Sleep(5 * time.Second)
			return ""
		}
	}
}

func XSSCheck(ctx context.Context, pool *pgxpool.Pool) {
	// get every extra_link
	rows, err := pool.Query(ctx, "SELECT bot_id, extra_links FROM bots")

	if err != nil {
		panic(err)
	}

	defer rows.Close()

	for rows.Next() {
		var botID pgtype.Text

		var extraLinks pgtype.JSONB

		err = rows.Scan(&botID, &extraLinks)

		if err != nil {
			panic(err)
		}

		var links map[string]string

		err = extraLinks.AssignTo(&links)

		if err != nil {
			panic(err)
		}

		for k := range links {
			if links[k] == "" {
				delete(links, k)
			}

			links[k] = strings.Trim(links[k], " ")

			// Internal links are not validated
			if strings.HasPrefix(k, "_") {
				fmt.Println("Internal link found, skipping validation")
				continue
			}

			// Validate URL
			links[k] = parseLink(k, links[k])

			fmt.Println("Parsed link for", k, "is", links[k])

			if links[k] == "" {
				delete(links, k)
			}
		}

		_, err = pool.Exec(ctx, "UPDATE bots SET extra_links = $1 WHERE bot_id = $2", links, botID.String)

		if err != nil {
			panic(err)
		}
	}
}
