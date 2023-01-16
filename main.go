package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/slack-go/slack"
	"github.com/spf13/viper"
)

type foodParkOption struct {
	name string
	url  string
}

const InputDateFormat = "2006-01-02"

func main() {
	now := time.Now()
	thursdayDate := now.Day() + ((7 + 4 - int(now.Weekday())) % 7)
	thursday := time.Now().Add(time.Hour * time.Duration((24 * (thursdayDate - now.Day()))))

	viper.SetEnvPrefix("fp")
	viper.AutomaticEnv()
	viper.SetDefault("url", "https://www.foodparkcam.com/whos-trading")
	viper.SetDefault("date_selector", "h1 > strong")
	viper.SetDefault("location_selector", "h2 > strong")
	viper.SetDefault("location_filter_value", "Cambridge Science Park")
	viper.SetDefault("anchor_selector", ".sqs-block-button-element")
	viper.SetDefault("outer_container_selector", "div.sqs-layout.sqs-grid-12.columns-12[data-type=page-section]")
	viper.SetDefault("target_date", thursday.Format(InputDateFormat))
	viper.SetDefault("date_format", "Mon 02 January")
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

	targetDate, _ := time.Parse(InputDateFormat, viper.GetString("target_date"))
	targetDateHeader := strings.ToUpper(targetDate.Format(viper.GetString("date_format")))

	var outerDiv *goquery.Selection = nil
	location := ""

	doc.Find(viper.GetString("date_selector")).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if strings.TrimSpace(s.Text()) != targetDateHeader {
			return true
		}

		for outerDiv == nil {
			s = s.Parent()
			if s.Is("html") {
				return true
			}

			if s.Is(viper.GetString("outer_container_selector")) {
				s.Find(viper.GetString("location_selector")).EachWithBreak(func(i int, s1 *goquery.Selection) bool {
					if strings.Contains(strings.ToLower(s1.Text()), strings.ToLower(viper.GetString("location_filter_value"))) {
						location = s1.Text()
						for outerDiv == nil {
							s1 = s1.Parent()
							if s1.Is("html") {
								return false
							}

							if s1.Is(viper.GetString("outer_container_selector")) {
								outerDiv = s1
								return false
							}
						}

						return false
					}

					return true
				})
			}
		}

		return outerDiv == nil
	})

	if outerDiv == nil {
		log.Fatal("Failed to find food park options from page data!")
		os.Exit(1)
	}

	var foodParkOptions []foodParkOption = []foodParkOption{}

	foodOptionsAnchorTags := outerDiv.Find(viper.GetString("anchor_selector"))

	foodOptionsAnchorTags.Each(func(i int, s1 *goquery.Selection) {
		url, exists := s1.Attr("href")

		if exists && url != "" && !strings.Contains(url, viper.GetString("target_date")) {
			log.Fatalf("Failed to find target date string %s in URL %s", viper.GetString("target_date"), url)
			os.Exit(1)
		}

		name := ""

		for name == "" {
			s1 = s1.Parent()

			if s1.Is("html") {
				log.Fatalf("Failed to find vendor name for %s", url)
				os.Exit(1)
			}

			prevSibling := s1.Prev()
			if len(prevSibling.Nodes) != 0 {
				name = strings.TrimSpace(prevSibling.Text())

				if url == "" {
					name += "*"
				}
			}
		}

		foodOption := foodParkOption{
			name: name,
			url:  url,
		}

		foodParkOptions = append(foodParkOptions, foodOption)
	})

	buttonBlocks := []slack.BlockElement{}
	walkUpOnly := false

	for _, opt := range foodParkOptions {
		style := slack.StylePrimary
		if opt.url == "" {
			walkUpOnly = true
			style = slack.StyleDefault
		}

		textBlock := slack.NewTextBlockObject("plain_text", opt.name, false, false)
		buttonBlock := slack.NewButtonBlockElement("", "", textBlock)
		buttonBlock.Style = style
		buttonBlock.URL = opt.url

		buttonBlocks = append(buttonBlocks, buttonBlock)
	}

	headerText := slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*foodPark Menus for %s at %s*", viper.GetString("target_date"), location), false, false)
	header := slack.NewSectionBlock(headerText, nil, nil)

	actions := slack.NewActionBlock("", buttonBlocks...)

	footerText := slack.NewTextBlockObject("mrkdwn", "* _denotes a truck not currently taking pre-orders._", false, false)
	footer := slack.NewSectionBlock(footerText, nil, nil)

	blocks := []slack.Block{header, actions}

	if walkUpOnly {
		blocks = append(blocks, footer)
	}

	message := &slack.WebhookMessage{
		Username: viper.GetString("slack_username"),
		Channel:  viper.GetString("slack_channel"),
		IconURL:  viper.GetString("slack_icon"),
		Blocks: &slack.Blocks{
			BlockSet: blocks,
		},
	}

	json.NewEncoder(os.Stdout).Encode(&message)

	err = slack.PostWebhook(viper.GetString("slack_webhook"), message)
	if err != nil {
		log.Fatalf("Failed to send Slack message: %s", err.Error())
		os.Exit(1)
	}
}
