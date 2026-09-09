package models

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

const DatafileDownloadURL = "api/v1/datafile/"

// DownloadTimeout is the timeout used for a single datafile download request.
// It needs to be generous since datafiles can be large and the client's
// default timeout (a few seconds) is far too short for a file transfer.
const DownloadTimeout = 1 * time.Hour

type Datafile struct {
	ID               int    `json:"id"`
	OriginalFilename string `json:"original_filename"`
	Size             int64  `json:"size"`
	Sha1             string `json:"sha1"`

	http.BaseModel
}

// Download downloads the datafile into destDir/OriginalFilename.
// OriginalFilename can itself contain subfolders (e.g. some DICOM exports
// use names like "DICOM/IM_0282"), in which case those intermediate
// directories are created too. If a file with the same size and sha1
// already exists at the final path, the download is skipped and
// skipped=true is returned. The file is streamed into a temporary ".part"
// file and atomically renamed on success, so an interrupted download never
// leaves a corrupt file at the final path.
//
// onProgress, if non-nil, is called with the number of bytes written on every
// underlying write, which callers can use to drive a progress indicator. It
// is not called when the download is skipped.
func (d *Datafile) Download(destDir string, onProgress func(n int64)) (path string, skipped bool, err error) {
	finalPath := filepath.Join(destDir, sanitizeFilePath(d.OriginalFilename))
	if err = os.MkdirAll(filepath.Dir(finalPath), 0o755); err != nil {
		return "", false, err
	}

	exists, err := d.matchesExisting(finalPath)
	if err != nil {
		return "", false, err
	}
	if exists {
		return finalPath, true, nil
	}

	tmpPath := finalPath + ".part"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", false, err
	}

	var w io.Writer = f
	if onProgress != nil {
		w = &progressWriter{w: f, onProgress: onProgress}
	}

	downloadPath := fmt.Sprintf("%s%d/download/", DatafileDownloadURL, d.ID)
	err = d.Client.Download(downloadPath, w, DownloadTimeout)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tmpPath)
		return "", false, err
	}

	if err = os.Rename(tmpPath, finalPath); err != nil {
		return "", false, err
	}
	return finalPath, false, nil
}

// sanitizeFilePath cleans a server-provided file path (OriginalFilename can
// embed subfolders, e.g. "DICOM/IM_0001") for safe use as a local
// filesystem path: it splits on "/" and "\" and drops any "", "." or ".."
// segment, so the result can never escape the directory it's joined onto
// (no path traversal via a crafted OriginalFilename).
func sanitizeFilePath(p string) string {
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
	kept := parts[:0]
	for _, s := range parts {
		if s == "" || s == "." || s == ".." {
			continue
		}
		kept = append(kept, s)
	}
	return filepath.Join(kept...)
}

// progressWriter forwards writes to w, reporting the number of bytes written
// on every call via onProgress.
type progressWriter struct {
	w          io.Writer
	onProgress func(n int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	if n > 0 {
		p.onProgress(int64(n))
	}
	return n, err
}

// matchesExisting reports whether a file already at path has the same size
// and sha1 as this datafile (size is checked first since it's much cheaper).
func (d *Datafile) matchesExisting(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if d.Size > 0 && info.Size() != d.Size {
		return false, nil
	}
	if d.Sha1 == "" {
		return true, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == d.Sha1, nil
}
