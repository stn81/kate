package log

import (
	"context"
	"os"
	"sync"
	"time"
)

var (
	OpenFlag = os.O_CREATE | os.O_APPEND | os.O_WRONLY
	OpenPerm = os.FileMode(0644)
)

var MaxRotateCount = 7

type Writer struct {
	location string
	lock     sync.RWMutex
	file     *os.File
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewWriter(location string) (*Writer, error) {
	file, err := os.OpenFile(location, OpenFlag, OpenPerm)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &Writer{
		location: location,
		file:     file,
		ctx:      ctx,
		cancel:   cancel,
	}

	w.wg.Add(1)
	go w.rotateLoop()

	return w, nil
}

func (w *Writer) Write(p []byte) (n int, err error) {
	w.lock.RLock()
	defer w.lock.RUnlock()

	return w.file.Write(p)
}

func (w *Writer) Sync() error {
	return w.file.Sync()
}

func (w *Writer) Close() error {
	w.cancel()
	w.wg.Wait()
	return w.file.Close()
}

func (w *Writer) rotateLoop() {
	defer func() {
		w.wg.Done()
	}()

	ticker := time.NewTicker(1 * time.Second)
	lastTickDay := time.Now().Day()
	for {
		select {
		case now := <-ticker.C:
			today := now.Day()
			if today != lastTickDay {
				lastTickDay = today
				w.rotate()
			}
		case <-w.ctx.Done():
			return
		}
	}

}

func (w *Writer) rotate() {
	var (
		dstFile = w.location + "." + w.getPrevDaySuffix(1)
		srcFile = w.location
		err     error
	)

	if exists, _ := w.isFileExists(srcFile); exists {
		//#ifdef DEBUG
		_, _ = os.Stderr.WriteString("rotating " + srcFile + " => " + dstFile + "\n")
		//#endif

		if err := os.Rename(srcFile, dstFile); err != nil {
			//#ifdef DEBUG
			_, _ = os.Stderr.WriteString("failed to rotate: srcFile=" + srcFile +
				", dstFile=" + dstFile + ", error=" + err.Error() + "\n")
			//#endif
			os.Exit(-1)
			return
		}
	}

	maxRotateFile := w.location + "." + w.getPrevDaySuffix(MaxRotateCount+1)
	if exists, _ := w.isFileExists(maxRotateFile); exists {
		//#ifdef DEBUG
		_, _ = os.Stderr.WriteString("removing " + maxRotateFile + "\n")
		//#endif
		_ = os.Remove(maxRotateFile)
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	if err = w.file.Close(); err != nil {
		//#ifdef DEBUG
		_, _ = os.Stderr.WriteString("ERROR: failed to close log file \"" +
			w.location + "\", reason=" + err.Error() + "\n")
		//#endif
		os.Exit(-1)
		return
	}

	w.file, err = os.OpenFile(w.location, OpenFlag, OpenPerm)
	if err != nil {
		//#ifdef DEBUG
		_, _ = os.Stderr.WriteString("ERROR: failed to rotate log file \"" +
			w.location + "\", reopen, reason=" + err.Error() + "\n")
		//#endif
		os.Exit(-1)
		return
	}
}

func (w *Writer) isFileExists(fileName string) (exists bool, err error) {
	if _, err = os.Stat(fileName); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (w *Writer) getPrevDaySuffix(i int) string {
	return time.Now().Add(-time.Hour * time.Duration(i) * 24).Format("20060102")
}
