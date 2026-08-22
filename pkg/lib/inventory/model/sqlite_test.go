package model

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"github.com/onsi/gomega"
	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"
)

func TestIsIOErrCode(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	g.Expect(isIOErrCode(sqlite3lib.SQLITE_IOERR)).To(gomega.BeTrue())
	g.Expect(isIOErrCode(sqlite3lib.SQLITE_IOERR_SHORT_READ)).To(gomega.BeTrue())
	g.Expect(isIOErrCode(sqlite3lib.SQLITE_IOERR_READ)).To(gomega.BeTrue())
	g.Expect(isIOErrCode(sqlite3lib.SQLITE_IOERR_WRITE)).To(gomega.BeTrue())
	g.Expect(isIOErrCode(sqlite3lib.SQLITE_CONSTRAINT)).To(gomega.BeFalse())
	g.Expect(isIOErrCode(sqlite3lib.SQLITE_CORRUPT)).To(gomega.BeFalse())
	g.Expect(isIOErrCode(sqlite3lib.SQLITE_OK)).To(gomega.BeFalse())
}

func TestIsIOErr(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	g.Expect(IsIOErr(nil)).To(gomega.BeFalse())
	g.Expect(IsIOErr(errors.New("connection refused"))).To(gomega.BeFalse())
	g.Expect(IsIOErr(errors.New("SQLITE_IOERR_SHORT_READ (522)"))).To(gomega.BeTrue())
	g.Expect(IsIOErr(liberr.Wrap(errors.New("SQLITE_IOERR")))).To(gomega.BeTrue())

	shortRead := sqliteErrorWithCode(sqlite3lib.SQLITE_IOERR_SHORT_READ)
	g.Expect(IsIOErr(shortRead)).To(gomega.BeTrue())
	g.Expect(IsIOErr(liberr.Wrap(shortRead))).To(gomega.BeTrue())

	constraint := sqliteErrorWithCode(sqlite3lib.SQLITE_CONSTRAINT)
	g.Expect(IsIOErr(constraint)).To(gomega.BeFalse())
}

func TestRebuildDiscardsCache(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	tmpDir, err := os.MkdirTemp("", "rebuild-test")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	defer os.RemoveAll(tmpDir)

	db := New(filepath.Join(tmpDir, "test.db"), &PlainObject{})
	err = db.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	defer db.Close(true)

	obj := &PlainObject{ID: 1, Name: "cached", Age: 20}
	g.Expect(db.Insert(obj)).ToNot(gomega.HaveOccurred())

	g.Expect(Rebuild(db)).ToNot(gomega.HaveOccurred())

	got := &PlainObject{ID: 1}
	err = db.Get(got)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())

	g.Expect(db.Insert(obj)).ToNot(gomega.HaveOccurred())
	got = &PlainObject{ID: 1}
	g.Expect(db.Get(got)).ToNot(gomega.HaveOccurred())
	g.Expect(got.Name).To(gomega.Equal("cached"))
}

func TestIOErrHealerRebuildsAfterThreshold(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	tmpDir, err := os.MkdirTemp("", "healer-test")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	defer os.RemoveAll(tmpDir)

	db := New(filepath.Join(tmpDir, "test.db"), &PlainObject{})
	err = db.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	defer db.Close(true)

	obj := &PlainObject{ID: 7, Name: "keep", Age: 3}
	g.Expect(db.Insert(obj)).ToNot(gomega.HaveOccurred())

	healer := &IOErrHealer{
		DB:        db,
		Log:       logging.WithName("test"),
		Threshold: 3,
	}
	ioErr := sqliteErrorWithCode(sqlite3lib.SQLITE_IOERR_SHORT_READ)

	g.Expect(healer.Observe(ioErr)).To(gomega.BeFalse())
	g.Expect(healer.Observe(ioErr)).To(gomega.BeFalse())
	got := &PlainObject{ID: 7}
	g.Expect(db.Get(got)).ToNot(gomega.HaveOccurred())

	g.Expect(healer.Observe(ioErr)).To(gomega.BeTrue())
	got = &PlainObject{ID: 7}
	err = db.Get(got)
	g.Expect(errors.Is(err, NotFound)).To(gomega.BeTrue())
}

func TestIOErrHealerResetsOnOtherErrors(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	tmpDir, err := os.MkdirTemp("", "healer-reset-test")
	g.Expect(err).ToNot(gomega.HaveOccurred())
	defer os.RemoveAll(tmpDir)

	db := New(filepath.Join(tmpDir, "test.db"), &PlainObject{})
	err = db.Open(true)
	g.Expect(err).ToNot(gomega.HaveOccurred())
	defer db.Close(true)

	obj := &PlainObject{ID: 8, Name: "keep", Age: 4}
	g.Expect(db.Insert(obj)).ToNot(gomega.HaveOccurred())

	healer := &IOErrHealer{
		DB:        db,
		Log:       logging.WithName("test"),
		Threshold: 3,
	}
	ioErr := sqliteErrorWithCode(sqlite3lib.SQLITE_IOERR)

	g.Expect(healer.Observe(ioErr)).To(gomega.BeFalse())
	g.Expect(healer.Observe(ioErr)).To(gomega.BeFalse())
	g.Expect(healer.Observe(errors.New("connection refused"))).To(gomega.BeFalse())
	g.Expect(healer.Observe(ioErr)).To(gomega.BeFalse())
	g.Expect(healer.Observe(ioErr)).To(gomega.BeFalse())

	got := &PlainObject{ID: 8}
	g.Expect(db.Get(got)).ToNot(gomega.HaveOccurred())
	g.Expect(got.Name).To(gomega.Equal("keep"))
}

func TestIOErrHealerNil(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	var healer *IOErrHealer
	g.Expect(healer.Observe(errors.New("SQLITE_IOERR"))).To(gomega.BeFalse())
}

// sqliteErrorWithCode builds a modernc.org/sqlite Error with the given result
// code. The Error fields are unexported, so tests set them through an identical
// layout.
func sqliteErrorWithCode(code int) error {
	err := &sqlite3.Error{}
	type sqliteError struct {
		msg  string
		code int
	}
	p := (*sqliteError)(unsafe.Pointer(err))
	p.msg = "sqlite io error"
	p.code = code
	// Keep the layout assertion so a driver change fails loudly.
	if reflect.TypeOf(*err).NumField() != 2 {
		panic("modernc.org/sqlite Error layout changed")
	}
	return err
}
