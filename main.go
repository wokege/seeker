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

var lastPubDate = time.Now()

func check() []rss.Item {
	resp, err := http.Get("https://vnexpress.net/rss/goc-nhin.rss")
	if err != nil {
		log.Println(err)
		return nil
	}
	defer resp.Body.Close()

	fp := rss.Parser{}
	feed, err := fp.Parse(resp.Body)
	_ = feed.Items

	var publish []rss.Item
	d := lastPubDate

	for _, v := range feed.Items {
		if v.PubDateParsed.After(d) {
			publish = append(publish, *v)
			if v.PubDateParsed.After(lastPubDate) {
				lastPubDate = *v.PubDateParsed
			}
		}
	}

	slices.Reverse(publish)

	return publish
}

func update(session *discordgo.Session, channel string) {
	for {
		items := check()
		if len(items) == 0 {
			time.Sleep(5 * time.Second)
			continue
		}

		for _, item := range items {
			send := discordgo.MessageSend{}
			embed := discordgo.MessageEmbed{}
			send.Embed = &embed

			embed.Title = item.Title

			embed.Description = strings.Split(item.Description, "</br>")[1]
			embed.URL = item.Link

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

			if _, err := session.ChannelMessageSendComplex(channel, &send); err != nil {
				log.Println(err)
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

	go update(discord, os.Getenv("CHANNEL_ID"))
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	discord.Close()
}
