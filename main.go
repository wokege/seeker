package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/mmcdole/gofeed/rss"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"
)

type Item struct {
	entry    rss.Item
	category string
}

var lastPub = make(map[string]time.Time)
var feeds = []string{
	"https://vnexpress.net/rss/goc-nhin.rss",
	"https://vnexpress.net/rss/tam-su.rss",
}

func getChannels() []string {
	c := strings.Split(os.Getenv("CHANNEL_ID"), ",")
	n := 0
	for _, channel := range c {
		v := strings.TrimSpace(channel)
		if v != "" {
			c[n] = v
			n++
		}
	}

	c = c[:n]
	return c
}

func check() []Item {
	var ret []Item

	for _, url := range feeds {
		resp, err := http.Get(url)
		if err != nil {
			log.Println(err)
			return nil
		}
		defer resp.Body.Close()

		fp := rss.Parser{}
		feed, err := fp.Parse(resp.Body)
		_ = feed.Items

		var publish []rss.Item
		d, ok := lastPub[url]
		if !ok {
			d = time.Now()
		}

		for _, v := range feed.Items {
			if v.PubDateParsed.After(d) {
				publish = append(publish, *v)
				if v.PubDateParsed.After(d) {
					d = *v.PubDateParsed
				}
			}
		}

		lastPub[url] = d

		slices.Reverse(publish)

		for _, item := range publish {
			ret = append(ret, Item{item, feed.Title})
		}
	}

	return ret
}

func update(session *discordgo.Session, channels []string) {
	for {
		items := check()
		if len(items) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		for _, w := range items {
			item := w.entry
			send := discordgo.MessageSend{}
			embed := discordgo.MessageEmbed{}
			footer := discordgo.MessageEmbedFooter{}
			send.Embeds = []*discordgo.MessageEmbed{
				&embed,
			}
			embed.Footer = &footer

			embed.Title = item.Title

			splitDesc := strings.Split(item.Description, "</br>")

			if len(splitDesc) >= 2 {
				embed.Description = splitDesc[1]
			} else {
				embed.Description = splitDesc[0]
			}

			embed.URL = item.Link
			footer.Text = w.category

			// imageLink URL
			re := regexp.MustCompile(`img src="(.*)"`)
			imageLink := re.FindStringSubmatch(item.Description)
			if imageLink != nil {
				imgRes, err := http.Get(imageLink[1])
				if err != nil {
					log.Println(err)
				} else {
					f := discordgo.File{}
					f.Reader = imgRes.Body
					f.Name = "thumb.jpg"
					send.File = &f

					i := discordgo.MessageEmbedImage{URL: "attachment://thumb.jpg"}
					embed.Image = &i
				}
			}

			for _, channel := range channels {
				if _, err := session.ChannelMessageSendComplex(channel, &send); err != nil {
					log.Printf("Error dispatching to channel %s : %s", channel, err)
				} else {
					log.Printf("Dispatched article '%s' to channel %s", item.Title, channel)
				}
			}

			if send.File != nil {
				send.File.Reader.(io.ReadCloser).Close()
			}
		}
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Print("Error loading .env file")
	}

	discord, err := discordgo.New("Bot " + os.Getenv("DISCORD_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}

	err = discord.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	channels := getChannels()
	log.Printf("Sending to channel %q", channels)

	go update(discord, channels)
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	discord.Close()
}
