package main

import (
	"bytes"
	"testing"
)

// TestStreamPrinter 驗證串流片段與完整訊息的輸出格式。
//
// 串流時片段逐字到來，若每個片段都印一次 "Agent: " 前綴，輸出會變得無法閱讀。
func TestStreamPrinter(t *testing.T) {
	cases := []struct {
		name string
		run  func(p *streamPrinter)
		want string
	}{
		{
			name: "純串流",
			run: func(p *streamPrinter) {
				p.Delta("Hel")
				p.Delta("lo")
				p.Flush()
			},
			want: "Agent: Hello\n",
		},
		{
			name: "串流後接完整訊息",
			run: func(p *streamPrinter) {
				p.Delta("Hel")
				p.Delta("lo")
				p.Reply("done")
			},
			want: "Agent: Hello\nAgent: done\n",
		},
		{
			name: "完整訊息自成一行",
			run: func(p *streamPrinter) {
				p.Reply("[Agent uses tool: read_file...]")
				p.Reply("done")
			},
			want: "Agent: [Agent uses tool: read_file...]\nAgent: done\n",
		},
		{
			name: "串流中斷後未補換行（由 Flush 收尾）",
			run: func(p *streamPrinter) {
				p.Delta("partial")
			},
			want: "Agent: partial",
		},
		{
			name: "Flush 不影響後續輸出",
			run: func(p *streamPrinter) {
				p.Delta("one")
				p.Flush()
				p.Delta("two")
				p.Flush()
			},
			want: "Agent: one\nAgent: two\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			tc.run(newStreamPrinter(&buf))

			if got := buf.String(); got != tc.want {
				t.Errorf("unexpected output:\ngot:  %q\nwant: %q", got, tc.want)
			}
		})
	}
}
