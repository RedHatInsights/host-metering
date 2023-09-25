package hostinfo

import (
	"path"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"redhat.com/milton/logger"
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

type CertWatcher interface {
	Close()
	Event() chan CertEvent
}

type INotifyCertWatcher struct {
	certPath   string
	event      chan CertEvent
	lastRemove time.Time
	lastWrite  time.Time
	watcher    *fsnotify.Watcher
}

func NewINotifyCertWatcher(certPath string) (*INotifyCertWatcher, error) {
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

	certWatcher := &INotifyCertWatcher{certPath: certPath, watcher: watcher}
	certWatcher.watch()
	logger.Infof("Watching cert directory %s for changes\n", dirPath)
	return certWatcher, nil
}

func (cw *INotifyCertWatcher) Event() chan CertEvent {
	return cw.event
}

func (cw *INotifyCertWatcher) Close() {
	cw.watcher.Close()
}

func (cw *INotifyCertWatcher) reportWriteEvent() {
	now := time.Now()
	if now.Sub(cw.lastWrite) < CertWatcherDelay {
		return
	}
	cw.lastWrite = now
	cw.event <- WriteEvent
}

func (cw *INotifyCertWatcher) reportRemoveEvent() {
	now := time.Now()
	if now.Sub(cw.lastRemove) < CertWatcherDelay {
		return
	}
	cw.lastRemove = now
	cw.event <- RemoveEvent
}

func (cw *INotifyCertWatcher) watch() <-chan CertEvent {
	cw.event = make(chan CertEvent)

	go func() {
		defer close(cw.event)
		for {
			select {
			case event, ok := <-cw.watcher.Events:
				if !ok {
					logger.Debugln("stopped watching cert directory")
					return
				}
				// ignore other files
				if path.Clean(event.Name) != path.Clean(cw.certPath) {
					continue
				}

				logger.Debugf("raw event: %s\n", event)

				if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
					cw.reportWriteEvent()
				}

				if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					cw.reportRemoveEvent()
				}
			case err, ok := <-cw.watcher.Errors:
				if !ok {
					logger.Debugln("stopped watching cert directory")
					return
				}
				logger.Infof("cert watcher error: %s\n", err)
			}
		}
	}()
	return cw.event
}
