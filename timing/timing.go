package timing

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type stat struct {
	count int64
	total time.Duration
	max   time.Duration
}

const minInterval = 500 * time.Millisecond

var (
	mu        sync.Mutex
	stats     = map[string]*stat{}
	lastFlush time.Time
	dirty     bool
)

func outPath() string {
	if p := os.Getenv("BENCH_FILE"); p != "" {
		return p
	}
	return "BENCHMARK.md"
}

// Track は関数の先頭で defer Track("name")() として使う
func Track(name string) func() {
	start := time.Now()
	return func() {
		d := time.Since(start) // 先に計測を確定させる

		mu.Lock()
		defer mu.Unlock()

		s := stats[name]
		if s == nil {
			s = &stat{}
			stats[name] = s
		}
		s.count++
		s.total += d
		if d > s.max {
			s.max = d
		}
		dirty = true

		if time.Since(lastFlush) >= minInterval {
			writeLocked()
		}
	}
}

// Flush は未書き出しの集計をBENCHMARK.mdに反映する
func Flush() {
	mu.Lock()
	defer mu.Unlock()
	if dirty {
		writeLocked()
	}
}

func writeLocked() {
	type row struct {
		name string
		avg  time.Duration
		*stat
	}
	rows := make([]row, 0, len(stats))
	for n, s := range stats {
		rows = append(rows, row{n, s.total / time.Duration(s.count), s})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].avg > rows[j].avg })

	var b strings.Builder
	b.WriteString("# BENCHMARK\n\n")
	fmt.Fprintf(&b, "最終更新: %s\n\n", time.Now().Format(time.RFC3339))
	b.WriteString("| 関数 | 呼び出し回数 | 平均 | 最大 |\n|---|---:|---:|---:|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| `%s` | %d | %v | %v |\n", r.name, r.count, r.avg, r.max)
	}

	path := outPath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err == nil {
		os.Rename(tmp, path) // 読み取り側が書きかけを見ないようにアトミックに置換
	}
	lastFlush = time.Now()
	dirty = false
}
