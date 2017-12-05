package client

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cheggaaa/pb"
)

const (
	TARINFO_HEADER = iota
	TARINFO_FILE
	TARINFO_PAD
	TARINFO_UPLOADED
	TARINFO_FINISHED
)

type tarInfo struct {
	info     os.FileInfo
	relPath  string
	linkName string
	path     string
	pad      int // number of zeros to pad at the end of the file entry

	headerBuf *bytes.Buffer
	pos       int64
	state     int
}

type TarFile struct {
	fileList  []*tarInfo
	blockSize int
	endPad    int
	padding   []byte
	closed    bool

	source   string
	progress *pb.ProgressBar
}

func (t *TarFile) writeHeader(p []byte, info *tarInfo) (int, error) {
	if info.headerBuf == nil {
		header, err := tar.FileInfoHeader(info.info, info.linkName)
		if err != nil {
			return 0, err
		}
		header.Name = info.relPath

		buf := &bytes.Buffer{}
		tarWriter := tar.NewWriter(buf)
		defer tarWriter.Close()
		err = tarWriter.WriteHeader(header)
		if err != nil {
			return 0, err
		}

		info.headerBuf = &bytes.Buffer{}
		io.Copy(info.headerBuf, buf)
	}

	size, err := info.headerBuf.Read(p)
	if err != nil && err != io.EOF {
		return 0, err
	}
	if info.headerBuf.Len() == 0 {
		info.headerBuf.Truncate(0)
	}
	return size, nil
}

func (t *TarFile) writeFile(p []byte, info *tarInfo) (int, error) {
	var f *os.File
	var err error

	if f, err = os.Open(info.path); err != nil {
		return 0, err
	}
	defer f.Close()
	// resuming
	if info.pos != 0 {
		if _, err = f.Seek(info.pos, os.SEEK_SET); err != nil {
			return 0, err
		}
	}
	if ret, err := f.Read(p); err != nil && err != io.EOF {
		return 0, err
	} else {
		return ret, nil
	}
}

func (t *TarFile) writePad(p []byte, info *tarInfo) (int, error) {
	buf := bytes.NewBuffer(t.padding[0:info.pad])
	if ret, err := buf.Read(p); err != nil && err != io.EOF {
		return 0, err
	} else {
		return ret, nil
	}
}

// write the trailing zeros of a tar file
func (t *TarFile) writeClose(p []byte) (n int, err error) {
	size := t.endPad
	buf := bytes.NewBuffer(make([]byte, size))
	if ret, err := buf.Read(p); err != nil && err != io.EOF {
		return 0, err
	} else {
		t.endPad -= ret
	}
	if t.endPad <= 0 {
		if t.progress != nil {
			t.progress.Finish()
		}
		t.closed = true
		return size, io.EOF
	}
	return size - t.endPad, nil
}

func (t *TarFile) AllocBar(pool *pb.Pool) *pb.ProgressBar {
	if t.progress == nil {
		t.progress = pb.New(len(t.fileList)).Prefix(fmt.Sprintf("Sending %s", t.source))
		pool.Add(t.progress)
	}
	return t.progress
}

func (t *TarFile) AddFile(info os.FileInfo, relPath, linkName, path string) {
	t.fileList = append(t.fileList, &tarInfo{
		info:     info,
		relPath:  relPath,
		linkName: linkName,
		path:     path,
		pad:      (t.blockSize - (int(info.Size()) % t.blockSize)) % t.blockSize,
	})
}

func (t *TarFile) Close() error {
	if !t.closed {
		if t.progress != nil {
			t.progress.Finish()
		}
	}
	t.closed = true
	return nil
}

func (t *TarFile) Read(p []byte) (n int, err error) {
	var (
		file   *tarInfo
		idx    int
		length = len(p)
	)

	if length == 0 {
		return
	}

	if t.closed {
		return 0, io.EOF
	}

	for idx, file = range t.fileList {
		if file.state != TARINFO_FINISHED {
			break
		}
		if idx == len(t.fileList)-1 {
			file = nil
			break
		}
	}

	defer func() {
		if err != nil && err != io.EOF {
			t.Close()
		}
	}()

	if file == nil {
		return t.writeClose(p)
	}

	for n < length && file.state != TARINFO_FINISHED {
		switch file.state {
		case TARINFO_HEADER:
			if ret, err := t.writeHeader(p, file); err != nil {
				return 0, err
			} else {
				n += ret
				p = p[n:]
				if file.headerBuf.Len() == 0 {
					if file.info.Mode().IsRegular() {
						file.state = TARINFO_FILE
					} else {
						file.state = TARINFO_UPLOADED
					}
				}
			}
		case TARINFO_FILE:
			if ret, err := t.writeFile(p, file); err != nil {
				return 0, err
			} else {
				file.pos += int64(ret)
				n += ret
				p = p[ret:]
				if file.pos >= file.info.Size() {
					file.pos = 0
					file.state = TARINFO_PAD
				}
			}
		case TARINFO_PAD:
			if file.pad == 0 {
				file.state = TARINFO_UPLOADED
			} else if ret, err := t.writePad(p, file); err != nil {
				return 0, err
			} else {
				file.pos += int64(ret)
				n += ret
				p = p[ret:]
				if file.pos >= int64(file.pad) {
					file.state = TARINFO_UPLOADED
				}
			}
		case TARINFO_UPLOADED:
			file.state = TARINFO_FINISHED
			if t.progress != nil {
				t.progress.Increment()
			}
		}
	}

	return n, nil
}

func NewTarFile(path string, blockSize int) *TarFile {
	return &TarFile{
		blockSize: blockSize,
		endPad:    blockSize * 2,
		source:    filepath.Base(path),
		padding:   make([]byte, blockSize),
	}
}
