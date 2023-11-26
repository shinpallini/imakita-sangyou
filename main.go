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
	ctx          context.Context
	client       *openai.Client
	relay        *nostr.Relay
}

func NewConfig(ctx context.Context) (*Config, error) {
	f, err := os.Open("config.json")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	err = json.NewDecoder(f).Decode(&cfg)
	if err != nil {
		return nil, err
	}
	cfg.ctx = ctx
	cfg.client = openai.NewClient(cfg.OpenAiApiKey)
	relay, err := nostr.RelayConnect(ctx, "wss://relay-jp.nostr.wirednet.jp")
	if err != nil {
		return nil, err
	}
	cfg.relay = relay
	return &cfg, nil

}

func (c *Config) setProfile() error {
	ev := nostr.Event{
		PubKey:    c.PublicKey,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindProfileMetadata,
		Tags:      nil,
		Content:   `{"name": "sangyou-bot", "about": "要約してほしい投稿に「3行で要約して」とこのbotにリプライを送ると、3行に要約します", "picture": ""}`,
	}
	ev.Sign(c.PrivateKey)

	_, err := c.relay.Publish(c.ctx, ev)
	if err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("profile update succeed")
	return nil
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// load config
	cfg, err := NewConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", cfg)
	cfg.setProfile()

	filters := []nostr.Filter{{
		Kinds: []int{nostr.KindTextNote, nostr.KindArticle},
		Tags:  nostr.TagMap{"p": []string{cfg.PublicKey}},
		Limit: 1,
	}}
	sub, err := cfg.relay.Subscribe(ctx, filters)
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
		sub, err := cfg.relay.Subscribe(ctx, filters)
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
