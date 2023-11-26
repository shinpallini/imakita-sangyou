package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sashabaranov/go-openai"
)

const (
	systemMessage = `あなたはユーザーからの入力を受け取り、その内容を3行に要約してください。
	それぞれ行の先頭には始まりを示す記号「・」を付けてください。`
)

type Config struct {
	PrivateKey   string `json:"privatekey`
	PublicKey    string `json:"publickey`
	OpenAiApiKey string `json:"openai_apikey"`
	client       *openai.Client
}

func NewConfig() *Config {
	f, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var cfg Config
	err = json.NewDecoder(f).Decode(&cfg)
	if err != nil {
		log.Fatal(err)
	}
	cfg.client = openai.NewClient(cfg.OpenAiApiKey)
	return &cfg

}

func (c *Config) setProfile() {

}

func (c *Config) summarize(content string) error {
	resp, err := c.client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT3Dot5Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemMessage,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: content,
				},
			},
		},
	)
	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", resp)
	return nil
}

func main() {
	// load config
	cfg := NewConfig()
	fmt.Printf("%+v\n", cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	relay, err := nostr.RelayConnect(ctx, "wss://relay-jp.nostr.wirednet.jp")
	if err != nil {
		log.Fatal(err)
	}

	var filters nostr.Filters
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	filters = []nostr.Filter{{
		Kinds: []int{nostr.KindTextNote, nostr.KindArticle},
		Tags:  nostr.TagMap{"p": []string{cfg.PublicKey}},
		Limit: 1,
	}}
	sub, err := relay.Subscribe(ctx, filters)
	if err != nil {
		log.Fatal(err)
	}

	for ev := range sub.Events {
		// handle returned event.
		// channel will stay open until the ctx is cancelled (in this case, context timeout)
		eventId := ev.Tags.GetLast([]string{"e"})
		filters := []nostr.Filter{{
			Kinds: []int{nostr.KindTextNote, nostr.KindArticle},
			IDs:   []string{"e", eventId.Value()},
			Limit: 1,
		}}
		sub, err := relay.Subscribe(ctx, filters)
		if err != nil {
			log.Fatal(err)
		}
		for evv := range sub.Events {
			fmt.Printf("%+v", evv)
		}

		fmt.Printf("%+v", ev)

		// cfg.summarize(ev.Content)
	}

}
