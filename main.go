package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/nlopes/slack"
	"github.com/spf13/viper"
)

type foodParkOption struct {
	name string
	url  string
}

func main() {
	now := time.Now()
	thursdayDate := now.Day() + ((7 + 4 - int(now.Weekday())) % 7)
	thursday := time.Now().Add(time.Hour * time.Duration((24 * (thursdayDate - now.Day()))))

	viper.SetEnvPrefix("fp")
	viper.AutomaticEnv()
	viper.SetDefault("url", "https://www.foodparkcam.com/whos-trading")
	viper.SetDefault("location_filter_query", "h2")
	viper.SetDefault("location_filter_value", "Cambridge Science Park")
	viper.SetDefault("anchor_filter_query", ".sqs-block-button-element")
	viper.SetDefault("break_filter_query", "spacer-block sqs-block-spacer")
	viper.SetDefault("target_date", thursday.Format("2006-01-02"))
	viper.SetDefault("slack_username", "foodPark")
	viper.BindEnv("slack_channel")
	viper.SetDefault("slack_icon", "https://foodparkcam.com/favicon.ico")
	viper.BindEnv("slack_webhook")

	if !viper.IsSet("slack_channel") || !viper.IsSet("slack_webhook") {
		log.Fatalf("FP_SLACK_CHANNEL or FP_SLACK_WEBHOOK missing! FP_SLACK_CHANNEL=%t; FP_SLACK_WEBHOOK=%t", viper.IsSet("slack_channel"), viper.IsSet("slack_webhook"))
		os.Exit(1)
	}

	response, err := http.Get(viper.GetString("url"))
	if err != nil {
		log.Fatalf("Failed to fetch Food Park page: %s", err.Error())
		os.Exit(1)
	}

	defer response.Body.Close()
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatalf("Failed to parse Food Park page: %s", err.Error())
		os.Exit(1)
	}

	var startingDiv *goquery.Selection = nil
	location := ""

	doc.Find(viper.GetString("location_filter_query")).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if strings.Contains(s.Text(), viper.GetString("location_filter_value")) {
			location = s.Text()
			startingDiv = s.Parent().Parent().Next()
			return false
		}
		return true
	})

	if startingDiv == nil {
		log.Fatal("Failed to find food park options from page data!")
		os.Exit(1)
	}

	possibleDivs := []*goquery.Selection{startingDiv}
	for !possibleDivs[len(possibleDivs)-1].Next().HasClass(viper.GetString("break_filter_query")) {
		possibleDivs = append(possibleDivs, possibleDivs[len(possibleDivs)-1].Next())
	}

	var foodParkOptions []foodParkOption = []foodParkOption{}

	// startingDiv.Children().Each(func(i int, s *goquery.Selection) {
	for _, s := range possibleDivs {
		foodOptionsAnchorTags := s.Find(viper.GetString("anchor_filter_query"))

		foodOptionsAnchorTags.Each(func(i int, s1 *goquery.Selection) {
			url, exists := s1.Attr("href")

			if !exists {
				log.Fatal("Anchor tag does not have a href attribute")
				os.Exit(1)
			}
			if !strings.Contains(url, viper.GetString("target_date")) {
				log.Fatalf("Failed to find target date string %s in URL %s", viper.GetString("target_date"), url)
				os.Exit(1)
			}

			name := s1.Parent().Parent().Parent().Prev().Text()

			foodOption := foodParkOption{
				name: name,
				url:  url,
			}

			foodParkOptions = append(foodParkOptions, foodOption)
		})
	}

	fallBackString := fmt.Sprintf("*foodPark Menus for %s at %s:*", viper.GetString("target_date"), location)
	attachmentActions := []slack.AttachmentAction{}

	for _, opt := range foodParkOptions {
		fallBackString += fmt.Sprintf("\n- <%s|%s>", opt.name, opt.url)
		attachmentActions = append(attachmentActions, slack.AttachmentAction{
			Name:  opt.name,
			Text:  opt.name,
			Style: "primary",
			Type:  "button",
			URL:   opt.url,
		})
	}

	message := &slack.WebhookMessage{
		Username: viper.GetString("slack_username"),
		Channel:  viper.GetString("slack_channel"),
		IconURL:  viper.GetString("slack_icon"),
		Attachments: []slack.Attachment{
			{
				Fallback: fallBackString,
				Title:    fmt.Sprintf("foodPark Menus for %s at %s", viper.GetString("target_date"), location),
				Actions:  attachmentActions,
			},
		},
	}

	err = slack.PostWebhook(viper.GetString("slack_webhook"), message)
	if err != nil {
		log.Fatalf("Failed to send Slack message: %s", err.Error())
		os.Exit(1)
	}
}
