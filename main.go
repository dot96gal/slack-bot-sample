package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	appToken := os.Getenv("SLACK_APP_TOKEN")
	if appToken == "" {
		slog.Error("SLACK_APP_TOKEN environment variable is required")
		os.Exit(1)
	}

	if !strings.HasPrefix(appToken, "xapp-") {
		slog.Error("SLACK_APP_TOKEN must have the prefix \"xapp-\"")
		os.Exit(1)
	}

	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if botToken == "" {
		slog.Error("SLACK_BOT_TOKEN environment variable is required")
		os.Exit(1)
	}

	if !strings.HasPrefix(botToken, "xoxb-") {
		slog.Error("SLACK_BOT_TOKEN must have the prefix \"xoxb-\"")
		os.Exit(1)
	}

	api := slack.New(
		botToken,
		slack.OptionDebug(true),
		slack.OptionLog(log.New(os.Stdout, "[api] ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(appToken),
	)

	client := socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "[socketmode] ", log.Lshortfile|log.LstdFlags)),
	)

	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				slog.Info("Connecting to Slack with Socket Mode...")
			case socketmode.EventTypeConnectionError:
				slog.Info("Connection failed. Retrying later...")
			case socketmode.EventTypeConnected:
				slog.Info("Connected to Slack with Socket Mode.")
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					slog.Info("Ignored", "event", evt)
					continue
				}

				slog.Info("Event received", "event", eventsAPIEvent)

				client.Ack(*evt.Request)

				// [NOTE] 1つのメッセージで複数のイベントが発生するため注意すること
				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						if ev.BotID != "" {
							break
						}

						_, _, err := client.PostMessage(
							ev.Channel,
							slack.MsgOptionTS(ev.TimeStamp), // Reply in thread
							slack.MsgOptionText("Yes, hello.", false),
						)
						if err != nil {
							slog.Error("failed posting message", "error", err)
						}
					case *slackevents.MessageEvent:
						if ev.BotID != "" {
							break
						}

						if ev.Type == "message" {
							_, _, err := client.PostMessage(
								ev.Channel,
								slack.MsgOptionTS(ev.TimeStamp), // Reply in thread
								slack.MsgOptionText(fmt.Sprintf("message: %s", ev.Message.Text), false),
							)
							if err != nil {
								slog.Error("failed posting message", "error", err)
							}
						}
					case *slackevents.MemberJoinedChannelEvent:
						slog.Info("user joined to channel", "user", ev.User, "channel", ev.Channel)
					}
				default:
					client.Debugf("unsupported Events API event received")
				}
			case socketmode.EventTypeHello:
				client.Debugf("Hello received!")
			default:
				slog.Error("Unexpected event type received", "type", evt.Type)
			}
		}
	}()

	client.Run()
}
