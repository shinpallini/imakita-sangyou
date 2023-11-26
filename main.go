package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/sashabaranov/go-openai"
)

const (
	systemMessage = `ユーザーからの入力を受け取った後、以下の指示に従って内容を3行に要約してください：
	1. 最初に「3行に要約したよ！」という文を含めてください。
	2. その後の各行の要約は、礼儀正しい「です・ます調」で記述してください。各文が丁寧な表現で終わるようにしてください。
	3. 各行の先頭には始まりを示す記号「・」を付けてください。
	
	例：
	ユーザーの入力: [ユーザーからの入力内容]
	要約:
	・3行に要約したよ！
	・[要約の内容1]
	・[要約の内容2]
	・[要約の内容3]
	`
)

type Config struct {
	PrivateKey   string   `json:"privatekey"`
	PublicKey    string   `json:"publickey"`
	OpenAiApiKey string   `json:"openai_apikey"`
	RelaysUrl    []string `json:"relays_url"`
	relays       map[string]*nostr.Relay
	ctx          context.Context
	client       *openai.Client
}

func NewConfig(ctx context.Context) (*Config, error) {
	// load config
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
	cfg.relays = make(map[string]*nostr.Relay)
	for _, v := range cfg.RelaysUrl {
		relay, err := nostr.RelayConnect(ctx, v)
		if err != nil {
			return nil, err
		}
		cfg.relays[v] = relay
	}
	return &cfg, nil

}

func (c *Config) setProfile() error {
	ev := nostr.Event{
		PubKey:    c.PublicKey,
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindProfileMetadata,
		Tags:      nil,
		Content:   `{"name": "sangyou-bot", "about": "今北産業！「3行でまとめて」とこのbotにリプライを送ると要約してくれます。", "picture": "https://robohash.org/npub1k2h9vmly9qdquvf75pvzyez3ysc5ctuj94yc5sc0vwkme297mvfsjxdl08?set=set4"}`,
	}
	ev.Sign(c.PrivateKey)

	for url, relay := range c.relays {
		_, err := relay.Publish(c.ctx, ev)
		if err != nil {
			fmt.Println(err)
			return err
		}
		fmt.Println("profile update succeed: ", url)
	}
	return nil
}

func (c *Config) summarize(content string) (string, error) {
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
		return "", err
	}
	fmt.Printf("%+v\n", resp)
	return resp.Choices[0].Message.Content, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	// defer cancel()
	// load config
	cfg, err := NewConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// fmt.Printf("%+v\n", cfg)
	cfg.setProfile()

	filters := []nostr.Filter{{
		Kinds: []int{nostr.KindTextNote},
		Tags:  nostr.TagMap{"p": []string{cfg.PublicKey}},
		Limit: 1,
	}}
	sub, err := cfg.relays["wss://relay-jp.nostr.wirednet.jp"].Subscribe(ctx, filters)
	if err != nil {
		log.Fatal(err)
	}
	isFirst := true
	for ev := range sub.Events {
		if isFirst {
			fmt.Println("skipped: first reqest event")
			isFirst = false
			continue
		}
		if strings.Contains(ev.Content, "3行でまとめて") {
			fmt.Println("skipped: do not contain command")
		}
		// handle returned event.
		// channel will stay open until the ctx is cancelled (in this case, context timeout)
		fmt.Printf("%+v\n", ev)
		userPubKey := ev.PubKey
		userEventId := ev.ID
		eventId := ev.Tags.GetFirst([]string{"e"}).Value()
		if eventId == "" {
			fmt.Println("event not found")
			continue
		}
		fmt.Println("eventId: ", eventId)
		filters := []nostr.Filter{{
			Kinds: []int{nostr.KindTextNote, nostr.KindArticle},
			IDs:   []string{eventId},
			Limit: 1,
		}}
		sub, err := cfg.relays["wss://relay-jp.nostr.wirednet.jp"].Subscribe(ctx, filters)
		if err != nil {
			fmt.Println(err)
		}
		// dispose of first subscribing content
		for ev := range sub.Events {
			fmt.Printf("%+v", ev.Content)
			if len(ev.Content) < 30 {
				fmt.Println("content is small")
				continue
			}
			summary, err := cfg.summarize(ev.Content)
			if err != nil {
				fmt.Println(err)
			}
			t := nostr.Tags{nostr.Tag{"e", ev.ID}, nostr.Tag{"e", userEventId}, nostr.Tag{"p", userPubKey}}
			// reply message
			postEv := nostr.Event{
				PubKey:    userPubKey,
				CreatedAt: nostr.Now(),
				Kind:      nostr.KindTextNote,
				Tags:      t,
				Content:   summary,
			}

			// calling Sign sets the event ID field and the event Sig field
			postEv.Sign(cfg.PrivateKey)
			_, err = cfg.relays["wss://relay-jp.nostr.wirednet.jp"].Publish(ctx, postEv)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println("published success")
		}
	}

}
