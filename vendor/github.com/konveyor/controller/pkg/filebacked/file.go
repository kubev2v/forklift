/*
File backing for collections.
File format:
   | kind: 2 (uint16)
   | size: 8 (uint64)
   | object: n (gob encoded)
   | ...
*/
package filebacked

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"github.com/google/uuid"
	liberr "github.com/konveyor/controller/pkg/error"
	"github.com/konveyor/controller/pkg/logging"
	"io"
	"os"
	pathlib "path"
	"runtime"
)

var log = logging.WithName("filebacked")

//
// File extension.
const (
	Extension = ".fb"
)

//
// Working Directory.
var WorkingDir = "/tmp"

//
// Writer.
type Writer struct {
	// File path.
	path string
	// File.
	file *os.File
	// Direct access index.
	index []int64
	// Dirty (needs flush).
	dirty bool
}

//
// Append (write) object.
func (w *Writer) Append(object interface{}) {
	// Lazy open.
	w.open()
	// Seek end.
	_, err := w.file.Seek(0, io.SeekEnd)
	if err != nil {
		panic(err)
	}
	// Update catalog.
	kind := catalog.add(object)
	// Encode object.
	var bfr bytes.Buffer
	encoder := gob.NewEncoder(&bfr)
	err = encoder.Encode(object)
	if err != nil {
		panic(err)
	}
	// Write entry.
	offset := w.writeEntry(kind, bfr)
	w.index = append(w.index, offset)
	w.dirty = true

	log.V(6).Info(
		"writer: appended object.",
		"path",
		w.path,
		"kind",
		kind)

	return
}

//
// Build a reader.
func (w *Writer) Reader(shared bool) (reader *Reader) {
	w.open()
	w.flush()
	if !shared {
		path := w.newPath()
		err := os.Link(w.path, path)
		if err != nil {
			panic(err)
		}
		reader = &Reader{
			index: w.index[:],
			path:  path,
		}
		runtime.SetFinalizer(
			reader,
			func(r *Reader) {
				r.Close()
			})
	} else {
		reader = &Reader{
			index: w.index[:],
			path:  w.path,
			file:  w.file,
		}
	}
	log.V(5).Info(
		"writer: reader created.",
		"path",
		w.path,
		"link",
		reader.path)

	return
}

//
// Close the writer.
func (w *Writer) Close() {
	defer func() {
		_ = os.Remove(w.path)
	}()
	if w.file == nil {
		return
	}
	_ = w.file.Close()
	log.V(5).Info(
		"writer: closed.",
		"path",
		w.path)
}

//
// Flush.
func (w *Writer) flush() {
	if !w.dirty {
		return
	}
	err := w.file.Sync()
	if err == nil {
		w.dirty = false
	} else {
		panic(err)
	}
}

//
// Open the writer.
func (w *Writer) open() {
	if w.file != nil {
		return
	}
	var err error
	w.path = w.newPath()
	w.file, err = os.Create(w.path)
	if err != nil {
		panic(err)
	}
	log.V(5).Info(
		"writer: opened.",
		"path",
		w.path)

	return
}

//
// Write entry.
func (w *Writer) writeEntry(kind uint16, bfr bytes.Buffer) (offset int64) {
	file := w.file
	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		panic(err)
	}
	// Write object kind.
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, kind)
	_, err = file.Write(b)
	if err != nil {
		panic(err)
	}
	// Write object encoded length.
	n := bfr.Len()
	b = make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(n))
	_, err = file.Write(b)
	if err != nil {
		panic(err)
	}
	// Write encoded object.
	nWrite, err := file.Write(bfr.Bytes())
	if err != nil {
		panic(err)
	}
	if n != nWrite {
		err = liberr.New("Write failed.")
	}
	log.V(6).Info(
		"writer: write entry.",
		"path",
		w.path,
		"kind",
		kind,
		"length",
		len(w.index))

	return
}

//
// New path.
func (w *Writer) newPath() string {
	uid, _ := uuid.NewUUID()
	name := uid.String() + Extension
	return pathlib.Join(WorkingDir, name)
}

//
// Reader.
type Reader struct {
	// File path.
	path string
	// File.
	file *os.File
	// Direct access index.
	index []int64
	// shared
	shared bool
}

//
// Length.
// Number of objects in the list.
func (r *Reader) Len() (length int) {
	return len(r.index)
}

//
// Get the object at index.
func (r *Reader) At(index int) (object interface{}) {
	// Lazy open.
	r.open()
	// Seek.
	offset := r.index[index]
	_, err := r.file.Seek(offset, io.SeekStart)
	if err != nil {
		panic(err)
	}
	// Read entry.
	kind, b := r.readEntry()
	// Decode object.
	bfr := bytes.NewBuffer(b)
	decoder := gob.NewDecoder(bfr)
	object, found := catalog.build(kind)
	if !found {
		panic(liberr.New("object not found in catalog."))
	}
	err = decoder.Decode(object)
	if err != nil {
		panic(err)
	}

	log.V(6).Info(
		"reader: read at index.",
		"path",
		r.path,
		"index",
		index)

	return
}

//
// Get the object at index.
func (r *Reader) AtWith(index int, object interface{}) {
	// Lazy open.
	r.open()
	// Seek.
	offset := r.index[index]
	_, err := r.file.Seek(offset, io.SeekStart)
	if err != nil {
		panic(err)
	}
	// Read entry.
	_, b := r.readEntry()
	// Decode object.
	bfr := bytes.NewBuffer(b)
	decoder := gob.NewDecoder(bfr)
	err = decoder.Decode(object)
	if err != nil {
		panic(err)
	}

	log.V(6).Info(
		"reader: read at index (with) object.",
		"path",
		r.path,
		"index",
		index)

	return
}

//
// Close the reader.
func (r *Reader) Close() {
	if r.shared {
		return
	}
	defer func() {
		_ = os.Remove(r.path)
	}()
	if r.file == nil {
		return
	}
	_ = r.file.Close()
	log.V(5).Info(
		"reader: closed.",
		"path",
		r.path)
}

//
// Read next entry.
func (r *Reader) readEntry() (kind uint16, bfr []byte) {
	file := r.file
	// Read object kind.
	b := make([]byte, 2)
	_, err := file.Read(b)
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
		return
	}
	kind = binary.LittleEndian.Uint16(b)
	// Read object encoded length.
	b = make([]byte, 8)
	_, err = file.Read(b)
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
		return
	}
	n := int64(binary.LittleEndian.Uint64(b))
	// Read encoded object.
	b = make([]byte, n)
	_, err = file.Read(b)
	if err != nil {
		if err != io.EOF {
			panic(err)
		}
		return
	}

	bfr = b

	return
}

//
// Open the reader.
func (r *Reader) open() {
	if r.shared || r.file != nil {
		return
	}
	// Open.
	var err error
	r.file, err = os.Open(r.path)
	if err != nil {
		panic(err)
	}

	log.V(5).Info(
		"reader: opened.",
		"path",
		r.path)

	return
}
