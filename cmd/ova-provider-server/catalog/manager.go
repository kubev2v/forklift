package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	liberr "github.com/konveyor/forklift-controller/pkg/lib/error"
	"github.com/konveyor/forklift-controller/pkg/lib/logging"
	"gopkg.in/yaml.v2"
)

const (
	DownloadFilename  = "vm.ova.incomplete"
	ApplianceFilename = "vm.ova"
)

type OVAConfig struct {
	URLs []string `yaml:"urls"`
}

func New(catalogPath string, configPath string, scanInterval int, prune bool, concurrent int) (m *Manager, err error) {
	m = &Manager{
		CatalogPath:            catalogPath,
		ConfigPath:             configPath,
		ScanInterval:           scanInterval,
		Prune:                  prune,
		MaxConcurrentDownloads: concurrent,
	}
	m.Log = logging.WithName("catalog")
	return
}

type Manager struct {
	Context                context.Context
	Log                    logging.LevelLogger
	CatalogPath            string
	ConfigPath             string
	ScanInterval           int
	Config                 OVAConfig
	Prune                  bool
	MaxConcurrentDownloads int
}

func (m *Manager) Run(ctx context.Context) (err error) {
	go func() {
		m.Log.Info("Started.")
		defer m.Log.Info("Stopped.")
		for {
			select {
			case <-time.After(m.interval()):
				done := m.reconcile()
				if done {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return
}

func (m *Manager) reconcile() (done bool) {
	err := m.config()
	if err != nil {
		m.Log.Error(err, "failed to load provider config", "path", m.ConfigPath)
		if errors.Is(err, &os.PathError{}) {
			// if the file contains invalid yaml it could be fixed
			// by a configmap update, but a filesystem error suggests
			// an unfixable problem.
			done = true
		}
		return
	}
	if m.Prune {
		m.Log.Info("Pruning appliances.")
		err = m.prune()
		if err != nil {
			m.Log.Error(err, "failed to prune appliances", "path", m.CatalogPath)
			done = true
			return
		}
	}

	wg := NewQueuingWaitGroup(m.MaxConcurrentDownloads)
	for _, url := range m.Config.URLs {
		if !m.present(url) {
			wg.Add()
			go func() {
				defer wg.Done()
				err = m.download(url)
				if err != nil {
					m.Log.Error(err, "failed to download appliance", "url", url)
					if !errors.Is(err, &os.PathError{}) {
						err = m.remove(url)
						if err != nil {
							m.Log.Error(err, "unable to clean up after failed download", "url", url)
						}
					}
				}
			}()
		}
	}
	wg.Wait()
	return
}

func (m *Manager) config() (err error) {
	file, err := os.Open(m.ConfigPath)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&m.Config)
	if err != nil {
		return
	}
	return
}

func (m *Manager) prune() error {
	remoteAppliances := make(map[string]bool)
	for _, url := range m.Config.URLs {
		remoteAppliances[string2hash(url)] = true
	}
	entries, err := os.ReadDir(m.CatalogPath)
	if err != nil {
		err = liberr.Wrap(err)
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !entry.IsDir() {
			continue
		}
		if !remoteAppliances[string2hash(entry.Name())] {
			dir := path.Join(m.CatalogPath, entry.Name())
			m.Log.Info("Pruning appliance directory not found in config.", "path", dir)
			paths := []string{path.Join(dir, ApplianceFilename), path.Join(dir, DownloadFilename), dir}
			for _, p := range paths {
				err = os.Remove(p)
				if err != nil {
					if !errors.Is(err, fs.ErrNotExist) {
						err = liberr.Wrap(err)
						m.Log.Error(err, "unable to prune appliance", "path", p)
					}
				}
			}
		}
	}
	return nil
}

func (m *Manager) download(url string) (err error) {
	applianceDir := m.applianceDir(url)
	err = os.MkdirAll(applianceDir, 0755)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	downloadPath := path.Join(applianceDir, DownloadFilename)
	file, err := os.Create(downloadPath)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	response, err := http.Get(url)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		err = liberr.New("unknown status", "code", response.StatusCode, "status", response.Status)
		return
	}

	reader := LoggingReader{
		Log:           logging.WithName("download"),
		Source:        url,
		Reader:        response.Body,
		ContentLength: response.ContentLength,
	}
	_, err = io.Copy(file, &reader)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	finalPath := path.Join(applianceDir, ApplianceFilename)
	err = os.Rename(downloadPath, finalPath)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

func (m *Manager) remove(url string) (err error) {
	err = os.RemoveAll(m.applianceDir(url))
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

func (m *Manager) applianceDir(url string) string {
	hash := string2hash(url)
	return path.Join(m.CatalogPath, hash)
}

func (m *Manager) interval() time.Duration {
	return time.Second * time.Duration(m.ScanInterval)
}

func (m *Manager) present(url string) bool {
	_, err := os.Stat(path.Join(m.applianceDir(url), ApplianceFilename))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false
		}
	}
	return true
}

func NewQueuingWaitGroup(limit int) *QueuingWaitGroup {
	q := &QueuingWaitGroup{}
	q.Reset(limit)
	return q
}

type QueuingWaitGroup struct {
	limit chan int
	wg    sync.WaitGroup
}

func (r *QueuingWaitGroup) Reset(limit int) {
	r.limit = make(chan int, limit)
	r.wg = sync.WaitGroup{}
}

func (r *QueuingWaitGroup) Add() {
	r.limit <- 1
	r.wg.Add(1)
}

func (r *QueuingWaitGroup) Done() {
	<-r.limit
	r.wg.Done()
}

func (r *QueuingWaitGroup) Wait() {
	r.wg.Wait()
}

type LoggingReader struct {
	io.Reader
	Log           logging.LevelLogger
	ContentLength int64
	Source        string
	BytesRead     int64
}

func (r *LoggingReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err != nil {
		return
	}
	r.BytesRead += int64(n)
	r.Log.V(10).Info("Read progress.", "source", r.Source, "size", r.ContentLength, "read", r.BytesRead)
	return
}

func string2hash(s string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
