package conversion

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/kubev2v/forklift/pkg/virt-v2v/config"
	"github.com/kubev2v/forklift/pkg/virt-v2v/utils"
)

const (
	Letters       = "abcdefghijklmnopqrstuvwxyz"
	LettersLength = len(Letters)
)

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
