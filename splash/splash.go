package splash

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/crgimenes/glaze"
)

const (
	// ログキューの上限。超えた分は破棄し、呼び出し側は絶対にブロックしない。
	logQueueSize = 1024
	// UIへの反映間隔。複数行をまとめて1回のDispatchで送る。
	flushInterval = 33 * time.Millisecond
	// 1回のDispatchで送る最大行数。
	maxBatch = 200
)

type Splash struct {
	window glaze.WebView

	logging *Logging

	mu        sync.Mutex
	destroyed bool

	closed atomic.Bool

	queue    chan string
	done     chan struct{}
	doneOnce sync.Once

	onClose func()
}

// New creates a new splash screen.
func New(onClose ...func()) (*Splash, error) {
	var s *Splash

	var callback func()
	if len(onClose) > 0 {
		callback = onClose[0]
	}

	w, err := glaze.NewWithOptions(glaze.Options{
		Debug:           false,
		HideUntilLoaded: true,

		OnClose: func() {
			if s != nil {
				s.handleUserClose()
			}
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create splash window: %w", err)
	}

	s = &Splash{
		window:  w,
		onClose: callback,
		queue:   make(chan string, logQueueSize),
		done:    make(chan struct{}),
	}

	w.SetTitle("Inquiry Explorer - Starting")
	w.SetSize(500, 300, glaze.HintFixed)
	w.SetHtml(splashHTML)

	go s.pump()

	return s, nil
}

// Run starts the splash event loop.
func (s *Splash) Run() {
	s.window.Run()
	s.shutdown()
	s.Destroy()
}

// StartLogging starts log capture and connects it to the splash.
func (s *Splash) StartLogging() error {
	if s.closed.Load() {
		return fmt.Errorf("splash is closed")
	}

	l := NewLogging()
	if err := l.Start(); err != nil {
		return fmt.Errorf("start logging: %w", err)
	}

	s.mu.Lock()
	if s.closed.Load() {
		s.mu.Unlock()
		go l.Stop()
		return fmt.Errorf("splash is closed")
	}
	s.logging = l
	s.mu.Unlock()

	// Printlnはノンブロッキングなので、stdout/stderrのパイプが詰まらない。
	go func() {
		for message := range l.Logs {
			s.Println(message)
		}
	}()

	return nil
}

// StopLogging restores stdout and stderr.
func (s *Splash) StopLogging() {
	s.mu.Lock()
	l := s.logging
	s.logging = nil
	s.mu.Unlock()

	if l != nil {
		l.Stop()
	}
}

// Println adds a log message to the splash screen.
// 呼び出し元をブロックしない。キューが満杯の場合はメッセージを破棄する。
func (s *Splash) Println(message string) {
	if s.closed.Load() {
		return
	}

	select {
	case s.queue <- message:
	default:
		// キュー満杯: 初期化処理を優先してログを捨てる
	}
}

// Printf adds a formatted log message.
func (s *Splash) Printf(format string, args ...any) {
	s.Println(fmt.Sprintf(format, args...))
}

// pump はキューのログをまとめてUIへ反映する専用goroutine。
// Dispatch/Evalの待ちはここだけで発生し、呼び出し元には影響しない。
func (s *Splash) pump() {
	batch := make([]string, 0, maxBatch)

	for {
		// 最初の1件を待つ
		select {
		case <-s.done:
			return
		case m := <-s.queue:
			batch = append(batch[:0], m)
		}

		// 溜まっている分をまとめて取り出す
	drain:
		for len(batch) < maxBatch {
			select {
			case m := <-s.queue:
				batch = append(batch, m)
			default:
				break drain
			}
		}

		s.flush(batch)

		// 更新頻度を制限してUIスレッドの負荷を抑える
		select {
		case <-s.done:
			return
		case <-time.After(flushInterval):
		}
	}
}

func (s *Splash) flush(batch []string) {
	if s.closed.Load() {
		return
	}

	data, err := json.Marshal(batch)
	if err != nil {
		return
	}

	script := "(function(l){l.forEach(function(m){window.addLog(m)})})(" + string(data) + ")"

	w := s.window
	w.Dispatch(func() {
		// Dispatch実行時点で閉じられていたら何もしない
		if s.closed.Load() {
			return
		}
		w.Eval(script)
	})
}

// shutdown はログ送信goroutineを止める(複数回呼んでも安全)。
func (s *Splash) shutdown() {
	s.closed.Store(true)
	s.doneOnce.Do(func() { close(s.done) })
}

// handleUserClose handles the native window close button.
// UIスレッドで呼ばれるため、重い処理やユーザーのコールバックは別goroutineで実行する。
func (s *Splash) handleUserClose() {
	if s.closed.Swap(true) {
		return
	}

	s.doneOnce.Do(func() { close(s.done) })
	s.Destroy()

	go func() {
		s.StopLogging()
		if s.onClose != nil {
			s.onClose()
		}
	}()
}

// Close terminates the splash event loop.
// 呼び出し元(初期化処理側)のgoroutineから安全に呼べる。
func (s *Splash) Close() {
	if s.closed.Swap(true) {
		return
	}

	s.doneOnce.Do(func() { close(s.done) })
	s.window.Terminate()
	s.StopLogging()
}

// Destroy releases native window resources.
func (s *Splash) Destroy() {
	s.mu.Lock()
	if s.destroyed {
		s.mu.Unlock()
		return
	}
	s.destroyed = true
	s.mu.Unlock()

	s.window.Destroy()
}
