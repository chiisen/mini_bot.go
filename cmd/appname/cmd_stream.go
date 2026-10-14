package main

import (
	"fmt"
	"io"
)

// streamPrinter 把「串流片段」與「完整訊息」輸出成一致且可讀的格式。
//
// 串流時片段逐字到來，不能每個片段都印一次 "Agent: " 前綴；而工具通知與 Notice
// 這類完整訊息應該自成一行。這個型別把規則集中在一處，也讓輸出格式能被測試。
type streamPrinter struct {
	out       io.Writer
	streaming bool
}

func newStreamPrinter(out io.Writer) *streamPrinter {
	return &streamPrinter{out: out}
}

// Delta 輸出一個串流片段（第一次輸出時補上前綴）。
func (p *streamPrinter) Delta(text string) {
	if !p.streaming {
		fmt.Fprint(p.out, "Agent: ")
		p.streaming = true
	}
	fmt.Fprint(p.out, text)
}

// Reply 輸出一則完整訊息；若正在串流中會先收尾換行。
func (p *streamPrinter) Reply(msg string) {
	p.Flush()
	fmt.Fprintf(p.out, "Agent: %s\n", msg)
}

// Flush 在串流結束時補上換行，讓後續輸出從新的一行開始。
func (p *streamPrinter) Flush() {
	if !p.streaming {
		return
	}
	fmt.Fprintln(p.out)
	p.streaming = false
}
