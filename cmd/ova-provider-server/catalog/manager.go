package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"gopkg.in/yaml.v2"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Manager struct {
	Context                context.Context
	Log                    logging.LevelLogger
	CatalogPath            string
	SourcePath             string
	ScanInterval           int
	Sources                []api.Source
	Prune                  bool
	MaxConcurrentDownloads int
	DownloadTimeout        int
	statuses               map[string]ApplianceStatus
	statusMutex            sync.RWMutex
}

func (m *Manager) GetStatuses() []ApplianceStatus {
	m.statusMutex.RLock()
	defer m.statusMutex.RUnlock()
	var list []ApplianceStatus
	for key := range m.statuses {
		list = append(list, m.statuses[key])
	}
	return list
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
		m.Log.Error(err, "failed to load provider config", "path", m.SourcePath)
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

	m.beginStaging()
	wg := NewQueuingWaitGroup(m.MaxConcurrentDownloads)
	for _, source := range m.Sources {
		url := source.URL
		if m.present(url) {
			m.markComplete(url)
		} else {
			m.markPending(url)
			wg.Add()
			go func() {
				defer wg.Done()
				dErr := m.download(url)
				if dErr != nil {
					m.Log.Error(dErr, "failed to download appliance", "url", url)
					m.markError(url, dErr)
					if !errors.Is(dErr, &os.PathError{}) {
						dErr = m.remove(url)
						if dErr != nil {
							m.Log.Error(dErr, "unable to clean up after failed download", "url", url)
						}
					}
				}
			}()
		}
	}
	wg.Wait()
	m.endStaging()
	return
}

func (m *Manager) beginStaging() {
	m.statusMutex.Lock()
	defer m.statusMutex.Unlock()
	for key := range m.statuses {
		status := m.statuses[key]
		status.staged = false
		m.statuses[key] = status
	}
}

func (m *Manager) endStaging() {
	m.statusMutex.Lock()
	defer m.statusMutex.Unlock()
	for key := range m.statuses {
		status := m.statuses[key]
		if !status.staged {
			delete(m.statuses, key)
		}
	}
}

func (m *Manager) config() (err error) {
	file, err := os.Open(m.SourcePath)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer func() {
		_ = file.Close()
	}()
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&m.Sources)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	return
}

func (m *Manager) prune() error {
	remoteAppliances := make(map[string]bool)
	for _, source := range m.Sources {
		url := source.URL
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
		if !remoteAppliances[entry.Name()] {
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

	httpclient := &http.Client{
		Timeout: time.Minute * time.Duration(m.DownloadTimeout),
	}
	response, err := httpclient.Get(url)
	if err != nil {
		err = liberr.Wrap(err)
		return
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("unexpected response code: %d, message: %s", response.StatusCode, response.Status)
		return
	}

	m.markInProgress(url, response.ContentLength, 0)
	reader := ProgressReader{
		Source:        url,
		Reader:        response.Body,
		ContentLength: response.ContentLength,
		ProgressFunc:  m.markInProgress,
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

func (m *Manager) markComplete(url string) {

	dir := m.applianceDir(url)
	filepath := path.Join(dir, ApplianceFilename)
	info, err := os.Stat(filepath)
	if err != nil {
		m.markError(url, err)
		return
	}
	m.statusMutex.Lock()
	modified := meta.NewTime(info.ModTime())
	m.statuses[url] = ApplianceStatus{
		Status:   StatusComplete,
		URL:      url,
		Progress: 100,
		Modified: &modified,
		Size:     info.Size(),
		staged:   true,
	}
	m.statusMutex.Unlock()
}

func (m *Manager) markError(url string, err error) {
	m.statusMutex.Lock()
	m.statuses[url] = ApplianceStatus{
		Status:   StatusError,
		URL:      url,
		Error:    err.Error(),
		Progress: 0,
		Size:     0,
		staged:   true,
	}
	m.statusMutex.Unlock()
}

func (m *Manager) markPending(url string) {
	m.statusMutex.Lock()
	if m.statuses[url].Status == "" {
		m.statuses[url] = ApplianceStatus{
			Status: StatusPending,
			URL:    url,
			staged: true,
		}
	}
	m.statusMutex.Unlock()
}

func (m *Manager) markInProgress(url string, length int64, read int64) {
	var progress int64
	if length > 0 {
		progress = (read * 100) / length
	}
	m.statusMutex.Lock()
	m.statuses[url] = ApplianceStatus{
		Status:   StatusInProgress,
		URL:      url,
		Progress: progress,
		Size:     read,
		staged:   true,
	}
	m.statusMutex.Unlock()
}

func string2hash(s string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
