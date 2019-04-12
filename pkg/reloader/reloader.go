package natsreloader

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Config represents the configuration of the reloader.
type Config struct {
	PidFile       string
	ConfigFile    string
	AuthFile      string
	MaxRetries    int
	RetryWaitSecs int
}

// Reloader monitors the state from a single server config file
// and sends signal on updates.
type Reloader struct {
	*Config

	// proc represents the NATS Server process which will
	// be signaled.
	proc *os.Process

	// pid is the last known PID from the NATS Server.
	pid int

	// quit shutsdown the reloader.
	quit func()

	// lastAppliedVersion is the digest of the contents
	// from the previous update.
	lastAppliedVersion []byte

	// lastAppliedAuthFileVersion is the digest of the contents
	// from the previous auth config file.
	lastAppliedAuthFileVersion []byte
}

// Run starts the main loop.
func (r *Reloader) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	r.quit = func() {
		cancel()
	}

	var (
		proc     *os.Process
		pid      int
		attempts int
	)
	for {
		pidfile, err := ioutil.ReadFile(r.PidFile)
		if err != nil {
			goto WaitAndRetry
		}

		pid, err = strconv.Atoi(string(pidfile))
		if err != nil {
			goto WaitAndRetry
		}

		proc, err = os.FindProcess(pid)
		if err != nil {
			goto WaitAndRetry
		}
		break

	WaitAndRetry:
		log.Printf("Error: %s\n", err)
		attempts++
		if attempts > r.MaxRetries {
			return fmt.Errorf("Too many errors attempting to find server process")
		}
		time.Sleep(time.Duration(r.RetryWaitSecs) * time.Second)
	}
	r.pid = pid
	r.proc = proc

	configWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer configWatcher.Close()

	// Follow configuration updates in the directory where
	// the config file is located and trigger reload when
	// it is either recreated or written into.
	if err := configWatcher.Add(path.Dir(r.ConfigFile)); err != nil {
		return err
	}

	// If we have an auth file configuration, watch it also.
	if r.AuthFile != "" {
		if err := configWatcher.Add(r.AuthFile); err != nil {
			return err
		}
	}

	attempts = 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-configWatcher.Events:
			log.Printf("Event: %+v \n", event)
			if event.Op != fsnotify.Write && event.Op != fsnotify.Create {
				continue
			}

			var (
				configDigest        []byte
				authFileDigest      []byte
				configUnchanged     bool
				authConfigUnchanged bool
			)
			configDigest, err = digest(r.ConfigFile)
			if err != nil {
				log.Printf("Error: %s\n", err)
				continue
			}
			if r.lastAppliedVersion != nil {
				configUnchanged = bytes.Equal(r.lastAppliedVersion, configDigest)
			}

			if r.AuthFile != "" {
				authFileDigest, err = digest(r.AuthFile)
				if err != nil {
					log.Printf("Error: %s\n", err)
					continue
				}
				if r.lastAppliedAuthFileVersion != nil {
					authConfigUnchanged = bytes.Equal(r.lastAppliedAuthFileVersion, authFileDigest)
				}
			}
			if configUnchanged && (r.AuthFile != "" && authConfigUnchanged) {
				// Skip since no changes
				continue
			}
			r.lastAppliedVersion = configDigest
			r.lastAppliedAuthFileVersion = authFileDigest
		case err := <-configWatcher.Errors:
			log.Printf("Error: %s\n", err)
			continue
		}

		// Configuration was updated, try to do reload for a few times
		// otherwise give up and wait for next event.
	TryReload:
		for {
			log.Println("Sending signal to server to reload configuration")
			err := r.proc.Signal(syscall.SIGHUP)
			if err != nil {
				log.Printf("Error during reload: %s\n", err)
				if attempts > r.MaxRetries {
					return fmt.Errorf("Too many errors attempting to signal server to reload")
				}
				log.Println("Wait and retrying after some time...")
				time.Sleep(time.Duration(r.RetryWaitSecs) * time.Second)
				attempts++
				continue TryReload
			}
			break TryReload
		}
	}

	return nil
}

func digest(file string) ([]byte, error) {
	h := sha256.New()
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return nil, nil
	}
	return h.Sum(nil), nil
}

// Stop shutsdown the process.
func (r *Reloader) Stop() error {
	log.Println("Shutting down...")
	r.quit()
	return nil
}

// NewReloader returns a configured NATS server reloader.
func NewReloader(config *Config) (*Reloader, error) {
	return &Reloader{
		Config: config,
	}, nil
}
