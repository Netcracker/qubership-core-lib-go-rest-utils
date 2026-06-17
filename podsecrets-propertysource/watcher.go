package podsecrets

import (
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

const debounceInterval = 300 * time.Millisecond

type Watcher struct {
	watcher *fsnotify.Watcher
	done    chan struct{}
}

func StartWatcher() (*Watcher, error) {
	dir := resolveDir()

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := fsw.Add(dir); err != nil {
		_ = fsw.Close()
		return nil, err
	}

	w := &Watcher{
		watcher: fsw,
		done:    make(chan struct{}),
	}
	go w.loop(dir)
	return w, nil
}

func (w *Watcher) loop(dir string) {
	debounce := time.NewTimer(debounceInterval)
	debounce.Stop()
	defer debounce.Stop()

	for {
		select {
		case _, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			debounce.Reset(debounceInterval)

		case <-debounce.C:
			if err := configloader.Refresh(); err != nil {
				logger.Warn("Pod-secrets refresh after change in %s failed: %s", dir, err.Error())
			} else {
				logger.Debug("Pod-secrets refreshed after change in %s", dir)
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			logger.Warn("Pod-secrets watcher error for %s: %s", dir, err.Error())

		case <-w.done:
			return
		}
	}
}

func (w *Watcher) Stop() {
	close(w.done)
	_ = w.watcher.Close()
}
