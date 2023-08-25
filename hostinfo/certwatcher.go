package hostinfo

import (
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type CertEvent int64

const (
	WriteEvent  CertEvent = 0
	RemoveEvent CertEvent = 1
)

const (
	// Consume similar events that occur within this time window
	CertWatcherDelay = 20 * time.Millisecond
)

type CertWatcher struct {
	certPath   string
	Event      chan CertEvent
	lastRemove time.Time
	lastWrite  time.Time
	watcher    *fsnotify.Watcher
}

func NewCertWatcher(certPath string) (*CertWatcher, error) {
	dirPath := filepath.Dir(certPath)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	// Watching the directory instead of the certificate to get events for
	// all its files and thus detect when cert is moved in/out, re-created ...
	err = watcher.Add(dirPath)
	if err != nil {
		return nil, err
	}

	certWatcher := &CertWatcher{certPath: certPath, watcher: watcher}
	certWatcher.watch()
	fmt.Printf("Watching cert directory %s for changes\n", dirPath)
	return certWatcher, nil
}

func (cw *CertWatcher) Close() {
	cw.watcher.Close()
}

func (cw *CertWatcher) reportWriteEvent() {
	now := time.Now()
	if now.Sub(cw.lastWrite) < CertWatcherDelay {
		return
	}
	cw.lastWrite = now
	cw.Event <- WriteEvent
}

func (cw *CertWatcher) reportRemoveEvent() {
	now := time.Now()
	if now.Sub(cw.lastRemove) < CertWatcherDelay {
		return
	}
	cw.lastRemove = now
	cw.Event <- RemoveEvent
}

func (cw *CertWatcher) watch() <-chan CertEvent {
	cw.Event = make(chan CertEvent)

	go func() {
		defer close(cw.Event)
		for {
			select {
			case event, ok := <-cw.watcher.Events:
				if !ok {
					fmt.Printf("stopped watching cert directory")
					return
				}
				// ignore other files
				if path.Clean(event.Name) != path.Clean(cw.certPath) {
					continue
				}

				fmt.Printf("raw event: %s\n", event)

				if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
					cw.reportWriteEvent()
				}

				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					cw.reportRemoveEvent()
				}
			case err, ok := <-cw.watcher.Errors:
				if !ok {
					fmt.Printf("stopped watching cert directory")
					return
				}
				fmt.Printf("cert watcher error: %s\n", err)
			}
		}
	}()
	return cw.Event
}
