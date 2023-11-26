package main

import (
	"context"
	"testing"
)

func TestSummarize(t *testing.T) {
	content := `Elm は JavaScript にコンパイルできる関数型プログラミング言語です。 ウェブサイトやウェブアプリケーションを作るのに役立ちます。Elm はシンプルであること、簡単に使えること、高品質であることを大切にしています。

	このガイドは以下のことを目指します。
	
	Elm によるプログラミングの基礎を身に着けてもらうこと
	The Elm Architecture を使ってインタラクティブなアプリケーションを作る方法をお見せすること
	あらゆる言語で使える法則やパターンを重視すること
	最終的にはあなたには Elm を使って素晴らしいウェブアプリをただ作れるようになるだけでなく、Elm をうまく使えるようになるための核となるアイディアやパターンを理解してもらえればと思います。
	
	Elm に対して様子見の立場である方も、Elm をちょっと試してみて実際に何かプロジェクトで使ってみると JavaScript のコードがいままでよりもうまく書けるようになっているはずです。 Elm で得られた知見はいろんなところで簡単に役立てることができます。`
	cfg, err := NewConfig(context.Background())
	if err != nil {
		t.Error(err)
	}
	out, err := cfg.summarize(content)
	if err != nil {
		t.Error(err)
	}
	t.Log(out)
}
