package splash

import (
	"bufio"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

const (
	logBufferSize = 1000
	maxLogSize    = 1024 * 1024

	// コンソール書き込みキューの上限。溢れたらコンソール出力を捨てる。
	consoleQueueSize = 4096

	// Stop 時に capture goroutine の終了を待つ最大時間。
	stopTimeout = 500 * time.Millisecond
)

type Logging struct {
	Logs chan string

	originalStdout *os.File
	originalStderr *os.File
	originalLog    io.Writer

	stdoutRead  *os.File
	stdoutWrite *os.File

	stderrRead  *os.File
	stderrWrite *os.File

	logRead  *os.File
	logWrite *os.File

	wg      sync.WaitGroup
	mu      sync.Mutex
	started bool
	stopped bool
}

func NewLogging() *Logging {
	return &Logging{
		Logs: make(chan string, logBufferSize),
	}
}

// Start redirects stdout, stderr and standard logger output.
func (l *Logging) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.started {
		return nil
	}
	if l.stopped {
		return os.ErrClosed
	}

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return err
	}

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		stdoutR.Close()
		stdoutW.Close()
		return err
	}

	logR, logW, err := os.Pipe()
	if err != nil {
		stdoutR.Close()
		stdoutW.Close()
		stderrR.Close()
		stderrW.Close()
		return err
	}

	l.originalStdout = os.Stdout
	l.originalStderr = os.Stderr
	l.originalLog = log.Writer()

	l.stdoutRead, l.stdoutWrite = stdoutR, stdoutW
	l.stderrRead, l.stderrWrite = stderrR, stderrW
	l.logRead, l.logWrite = logR, logW

	os.Stdout = stdoutW
	os.Stderr = stderrW
	log.SetOutput(logW)

	l.started = true

	l.wg.Add(3)

	go l.capture(stdoutR, newAsyncWriter(l.originalStdout))
	go l.capture(stderrR, newAsyncWriter(l.originalStderr))
	go l.capture(logR, newAsyncWriter(l.originalLog))

	return nil
}

// capture reads output from a pipe.
// パイプの読み取りは絶対に止めない(止めると書き込み側がブロックする)。
func (l *Logging) capture(reader *os.File, console *asyncWriter) {
	defer l.wg.Done()
	defer reader.Close()
	defer console.Close()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), maxLogSize)

	for scanner.Scan() {
		message := scanner.Text()

		// コンソールへは非同期(ノンブロッキング)
		console.WriteLine(message)

		// スプラッシュへもノンブロッキング
		select {
		case l.Logs <- message:
		default:
		}
	}

	// 行が長すぎる場合など: 読み取りを止めずに残りを捨てる。
	// これをしないと、書き込み側がブロックしてしまう。
	if err := scanner.Err(); err != nil {
		console.WriteLine("[LOGGER ERROR] " + err.Error())
		_, _ = io.Copy(io.Discard, reader)
	}
}

// Stop restores original output destinations.
func (l *Logging) Stop() {
	l.mu.Lock()

	if !l.started || l.stopped {
		l.mu.Unlock()
		return
	}

	l.stopped = true

	os.Stdout = l.originalStdout
	os.Stderr = l.originalStderr
	log.SetOutput(l.originalLog)

	_ = l.stdoutWrite.Close()
	_ = l.stderrWrite.Close()
	_ = l.logWrite.Close()

	l.mu.Unlock()

	// capture の終了を待つが、永遠には待たない。
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(l.Logs)
	case <-time.After(stopTimeout):
		// 書き込み側FDが他で保持されている場合など。
		// 呼び出し元を止めず、バックグラウンドで後始末する。
		go func() {
			<-done
			close(l.Logs)
		}()
	}
}

// asyncWriter は元のコンソールへの書き込みを別goroutineに逃がす。
// コンソールが遅くても capture(=パイプ読み取り)は止まらない。
type asyncWriter struct {
	w     io.Writer
	queue chan string
	done  chan struct{}
	once  sync.Once
}

func newAsyncWriter(w io.Writer) *asyncWriter {
	a := &asyncWriter{
		w:     w,
		queue: make(chan string, consoleQueueSize),
		done:  make(chan struct{}),
	}

	if w == nil {
		close(a.done)
		return a
	}

	go func() {
		defer close(a.done)
		for m := range a.queue {
			_, _ = io.WriteString(a.w, m+"\n")
		}
	}()

	return a
}

// WriteLine はノンブロッキング。キュー満杯なら破棄する。
func (a *asyncWriter) WriteLine(message string) {
	if a.w == nil {
		return
	}
	select {
	case a.queue <- message:
	default:
	}
}

// Close はキューを閉じる(待たない)。
func (a *asyncWriter) Close() {
	a.once.Do(func() { close(a.queue) })
}
