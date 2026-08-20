package conversion

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"time"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

const (
	Letters       = "abcdefghijklmnopqrstuvwxyz"
	LettersLength = len(Letters)
)

type blockDevOpener func(path string) error
type sleepFunc func(time.Duration)

const blockDevMaxAttempts = 10

type Disk struct {
	// The path to the connected disk
	Path string
	// The link is used to connect the attached disk to the virt-v2v output
	Link       string
	IsBlockDev bool
	appConfig  *config.AppConfig
	fileSystem utils.FileSystem
}

func NewDisk(cfg *config.AppConfig, diskPath string) (*Disk, error) {
	var isBlockDev = true
	if filepath.Dir(diskPath) == filepath.Dir(config.FS) {
		isBlockDev = false
		diskPath = filepath.Join(diskPath, "disk.img")
	}
	if isBlockDev {
		if err := waitForBlockDevice(diskPath); err != nil {
			return nil, err
		}
	}
	disk := Disk{
		Path:       diskPath,
		IsBlockDev: isBlockDev,
		appConfig:  cfg,
		fileSystem: utils.FileSystemImpl{},
	}
	link, err := disk.createLink()
	if err != nil {
		return nil, err
	}
	disk.Link = link

	return &disk, nil
}

// waitForBlockDevice polls the block device until it is ready, retrying
// transient errors (ENXIO, ENOENT, EIO) with exponential backoff. Permanent
// errors fail immediately.
func waitForBlockDevice(path string) error {
	return waitForBlockDeviceWith(path, blockDevMaxAttempts, func(p string) error {
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		return f.Close()
	}, time.Sleep)
}

func waitForBlockDeviceWith(path string, maxAttempts int, open blockDevOpener, sleep sleepFunc) error {
	backoff := 500 * time.Millisecond
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := open(path)
		if err == nil {
			if attempt > 1 {
				fmt.Printf("Block device %s ready after %d attempts\n", path, attempt)
			}
			return nil
		}
		lastErr = err
		if !isTransientBlockDevErr(err) {
			return fmt.Errorf("block device %s: %w", path, err)
		}
		fmt.Printf("Block device %s not ready (attempt %d/%d): %v\n", path, attempt, maxAttempts, err)
		if attempt < maxAttempts {
			sleep(backoff)
			backoff = min(backoff*2, 30*time.Second)
		}
	}
	return fmt.Errorf("block device %s not ready after %d attempts: %w", path, maxAttempts, lastErr)
}

func isTransientBlockDevErr(err error) bool {
	return errors.Is(err, syscall.ENXIO) ||
		errors.Is(err, syscall.ENOENT) ||
		errors.Is(err, syscall.EIO)
}

func (d *Disk) getDiskName() string {
	if d.appConfig.NewVmName != "" {
		return d.appConfig.NewVmName
	}
	return d.appConfig.VmName
}

func (d *Disk) createLink() (string, error) {
	diskNum, err := d.getDiskNumber()
	if err != nil {
		return "", err
	}
	diskName := d.getDiskName()
	diskLink := filepath.Join(
		d.appConfig.Workdir,
		fmt.Sprintf("%s-sd%s", diskName, d.genName(diskNum+1)),
	)
	if err = d.fileSystem.Symlink(d.Path, diskLink); err != nil {
		fmt.Println("Error creating disk link ", err)
		return "", err
	}
	return diskLink, nil
}

func (d *Disk) getDiskNumber() (int, error) {
	re := regexp.MustCompile(`\d+`)
	return strconv.Atoi(re.FindString(d.Path))
}

func (d *Disk) genName(diskNum int) string {
	if diskNum <= 0 {
		return ""
	}
	index := (diskNum - 1) % LettersLength
	cycles := (diskNum - 1) / LettersLength
	return d.genName(cycles) + string(Letters[index])
}
