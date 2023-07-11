package models

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agoraHttp "github.com/GyroTools/gtagora-connector-go/internals/http"
	uuid "github.com/nu7hatch/gouuid"
)

const (
	UPLOAD_CHUCK_SIZE = 100 * 1024 * 1024
	MAX_ZIP_SIZE      = 1024 * 1024 * 1024
	STATE_UPLOADING   = 1
	STATE_CHECKING    = 2
	STATE_ANALYZING   = 3
	STATE_IMPORTING   = 4
	STATE_FINISHED    = 5
	STATE_ERROR       = -1

	ImportPackageURL = "/api/v1/import/"
)

type ImportPackage struct {
	CompleteDate     string `json:"complete_date"`
	CreatedDate      string `json:"created_date"`
	Error            string `json:"error"`
	ExtractZipFiles  bool   `json:"extract_zip_files"`
	Id               int    `json:"id"`
	ImportFile       string `json:"import_file"`
	ImportParameters bool   `json:"import_parameters"`
	IsComplete       bool   `json:"is_complete"`
	ModifiedDate     string `json:"modified_date"`
	NofRetries       int    `json:"nof_retries"`
	State            int    `json:"state"`
	TargetId         int    `json:"target_id"`
	TargetType       string `json:"target_type"`
	TimelineItems    []int  `json:"timeline_items"`
	User             int    `json:"user"`

	agoraHttp.BaseModel
	chunksToUpload int `json:"-"`
	chunksUploaded int `json:"-"`
}

type FlowFile struct {
	ID                  int           `json:"id"`
	Chunks              []interface{} `json:"chunks"`
	Identifier          string        `json:"identifier"`
	OriginalFilename    string        `json:"original_filename"`
	TotalSize           int           `json:"total_size"`
	TotalChunks         int           `json:"total_chunks"`
	ContentHash         string        `json:"content_hash"`
	OriginalSourcePaths interface{}   `json:"original_source_paths"`
	TotalChunksUploaded int           `json:"total_chunks_uploaded"`
	State               int           `json:"state"`
	Created             string        `json:"created"`
	Updated             string        `json:"updated"`
}

type UploadFile struct {
	SourcePath string
	TargetPath string
	Delete     bool
	Size       int64
}

func (importPackage *ImportPackage) Upload(input_files []string, progressChan chan int) error {
	filesToUpload, filesToZip, err := analysePaths(input_files)
	if err != nil {
		return err
	}
	importPackage.chunksUploaded = 0
	totalSize := getTotalSize(filesToUpload, filesToZip)
	uploadedSize := int64(0)

	requestUrl := importPackage.Client.GetUrl(fmt.Sprintf("/api/v1/import/%d/upload/", importPackage.Id))

	// we have 2 threadpools here. One performs the large file upload and the zipping in parallel. One performs a parallel file upload
	parallel_uploads := 3
	fake := false

	apiKey, err := importPackage.Client.GetApiKey()
	if err != nil {
		return err
	}

	// Adding routines to workgroup and running then
	fileCh := make(chan UploadFile)
	uploadBytesCh := make(chan int64)
	wg := new(sync.WaitGroup)

	go func() {
		for bytes := range uploadBytesCh {
			uploadedSize += bytes
			progress := int(100 * uploadedSize / totalSize)
			if progress >= 100 {
				progress = 99
			}
			progressChan <- progress
		}
	}()

	for i := 0; i < parallel_uploads; i++ {
		wg.Add(1)
		go uploadWorker(fileCh, uploadBytesCh, requestUrl, apiKey, fake, wg)
	}

	temp_dir, err := ioutil.TempDir("", "agora_app")
	defer os.RemoveAll(temp_dir)
	if err != nil {
		return err
	}

	wg_upload_zip := new(sync.WaitGroup)
	wg_upload_zip.Add(2)

	go uploadFiles(fileCh, requestUrl, apiKey, filesToUpload, wg_upload_zip)
	go zipAndUpload(fileCh, requestUrl, apiKey, filesToZip, temp_dir, wg_upload_zip)
	wg_upload_zip.Wait()

	// Closing channel (waiting in goroutines won't continue any more)
	close(fileCh)

	// Waiting for all goroutines to finish (otherwise they die as main routine dies)
	wg.Wait()

	return nil
}

func (importPackage *ImportPackage) Complete(targetFolderId int, jsonImportFile string, extractZipFile bool, wg *sync.WaitGroup) error {
	path := fmt.Sprintf("/api/v1/import/%d/complete/", importPackage.Id)

	data := map[string]string{}
	if jsonImportFile != "" {
		data["import_file"] = jsonImportFile
	}
	if targetFolderId > 0 {
		data["folder"] = fmt.Sprintf("%d", targetFolderId)
	}
	if extractZipFile {
		data["extract_zip_files"] = "true"
	}
	json_data, err := json.Marshal(data)
	if err != nil {
		if wg != nil {
			defer wg.Done()
		}
		return err
	}

	resp, err := importPackage.Client.Post(path, bytes.NewBuffer(json_data), -1)
	if err != nil {
		if wg != nil {
			defer wg.Done()
		}
		return err
	}
	if resp.StatusCode != 204 {
		if wg != nil {
			defer wg.Done()
		}
		return errors.New(fmt.Sprintf("the \"complete\" request was invalid. http status = %d", resp.StatusCode))
	}

	if wg != nil {
		// wait for completion
		timeout := time.Duration(1 * time.Hour)
		go importPackage.wait(timeout, wg)
	}

	return nil
}

func (importPackage *ImportPackage) update() error {
	requestUrl := importPackage.Client.GetUrl(fmt.Sprintf("%s%d/", ImportPackageURL, importPackage.Id))
	err := importPackage.Client.GetAndParse(requestUrl, importPackage)
	if err != nil {
		return err
	}
	return nil
}

func (importPackage *ImportPackage) wait(timeout time.Duration, wg *sync.WaitGroup) error {
	defer wg.Done()
	if importPackage.State == STATE_FINISHED || importPackage.State == STATE_ERROR {
		return nil
	}

	startTime := time.Now()
	timeoutDuration := time.Duration(timeout) * time.Second
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(timeoutDuration):
			return errors.New("upload progress timeout")
		case <-ticker.C:
			err := importPackage.update()
			if err != nil {
				return err
			}
			if importPackage.State == STATE_FINISHED || importPackage.State == STATE_ERROR || importPackage.IsComplete {
				return nil
			}
		}
		if time.Since(startTime) > timeoutDuration {
			return errors.New("upload progress timeout")
		}
	}
}

func analysePaths(paths []string) ([]UploadFile, []UploadFile, error) {
	var filesToUpload []UploadFile
	var filesToZip []UploadFile
	for _, file := range paths {
		fileInfo, err := os.Stat(file)
		if os.IsNotExist(err) {
			return nil, nil, errors.New(fmt.Sprintf("the file \"%s\" does not exist", file))
		} else if err != nil {
			continue
		}
		if fileInfo.IsDir() {
			filepath.Walk(file, func(path string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					relativePath := path
					if strings.HasPrefix(path, file) {
						relativePath = path[len(file):]
					}
					relativePath = strings.Replace(relativePath, "\\", "/", -1)
					relativePath = strings.TrimPrefix(relativePath, "/")

					if info.Size() < UPLOAD_CHUCK_SIZE {
						filesToZip = append(filesToZip, UploadFile{SourcePath: strings.Replace(path, "\\", "/", -1), TargetPath: relativePath, Delete: false, Size: info.Size()})
					} else {
						filesToUpload = append(filesToUpload, UploadFile{SourcePath: strings.Replace(path, "\\", "/", -1), TargetPath: relativePath, Delete: false, Size: info.Size()})
					}
				}
				return nil
			})
		} else {
			absPath, err := filepath.Abs(file)
			if err != nil {
				absPath = file
			}
			if fileInfo.Size() < UPLOAD_CHUCK_SIZE {
				filesToZip = append(filesToZip, UploadFile{SourcePath: absPath, TargetPath: filepath.Base(file), Delete: false, Size: fileInfo.Size()})
			} else {
				filesToUpload = append(filesToUpload, UploadFile{SourcePath: absPath, TargetPath: filepath.Base(file), Delete: false, Size: fileInfo.Size()})
			}
		}
	}
	return filesToUpload, filesToZip, nil
}

func getTotalSize(filesToUpload []UploadFile, filesToZip []UploadFile) int64 {
	siz := int64(0)
	for _, file := range filesToZip {
		siz += file.Size
		siz += int64(len(file.TargetPath))
		siz += 150 // header size (approximate)
	}

	for _, file := range filesToUpload {
		siz += file.Size
	}
	return siz
}

func uploadWorker(fileChan chan UploadFile, uploadBytesCh chan int64, request_url string, api_key string, fake bool, wg *sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	for file := range fileChan {
		uploadFile(uploadBytesCh, request_url, api_key, file, fake)
	}
}

func uploadFile(uploadBytesCh chan int64, request_url string, api_key string, file UploadFile, fake bool) error {
	buffer := make([]byte, UPLOAD_CHUCK_SIZE)
	fileInfo, err := os.Stat(file.SourcePath)

	if file.Delete {
		defer os.Remove(file.SourcePath)
	}

	if err != nil {
		return err
	}
	filesize := fileInfo.Size()
	nof_chunks := int(math.Ceil(float64(filesize) / float64(UPLOAD_CHUCK_SIZE)))

	client := &http.Client{}
	r, err := os.Open(file.SourcePath)
	if err != nil {
		return err
	}
	uuid, _ := uuid.NewV4()

	chunk_failed := false
	for i := 0; i < nof_chunks; i++ {
		n, err := r.Read(buffer)
		if err != nil {
			chunk_failed = true
			break
		}
		chunk := bytes.NewReader(buffer[0:n])

		//prepare the reader instances to encode
		values := map[string]io.Reader{
			"file":                 chunk, // lets assume its this file
			"description":          strings.NewReader(""),
			"flowChunkNumber":      strings.NewReader(fmt.Sprintf("%d", i)),
			"flowChunkSize":        strings.NewReader(fmt.Sprintf("%d", UPLOAD_CHUCK_SIZE)),
			"flowCurrentChunkSize": strings.NewReader(fmt.Sprintf("%d", n)),
			"flowTotalSize":        strings.NewReader(fmt.Sprintf("%d", filesize)),
			"flowIdentifier":       strings.NewReader(uuid.String()),
			"flowFilename":         strings.NewReader(file.TargetPath),
			"flowRelativePath":     strings.NewReader(file.TargetPath),
			"flowTotalChunks":      strings.NewReader(fmt.Sprintf("%d", nof_chunks)),
		}

		cancel := make(chan struct{})
		nrBytesSent := int64(0)
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					sendBytes := int64(n) / 20
					if nrBytesSent+sendBytes >= int64(n) {
						return
					}
					uploadBytesCh <- sendBytes
					nrBytesSent += sendBytes
				case <-cancel:
					// Task cancellation requested
					return
				}
			}
		}()

		err = uploadChunk(client, request_url, api_key, values, filepath.Base(file.SourcePath), fake)
		if err != nil {
			chunk_failed = true
			break
		}
		close(cancel)
		uploadBytesCh <- int64(n) - nrBytesSent
	}
	r.Close()
	if chunk_failed {
		return err
	}
	return nil
}

func uploadChunk(client *http.Client, url string, api_key string, values map[string]io.Reader, filename string, fake bool) (err error) {
	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range values {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		// Add an image file
		_, ok := r.(*bytes.Reader)
		if ok {
			if fw, err = w.CreateFormFile(key, filename); err != nil {
				return
			}
		} else {
			// Add other fields
			if fw, err = w.CreateFormField(key); err != nil {
				return
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			return err
		}

	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())
	if api_key != "" {
		req.Header.Set("Authorization", "X-Agora-Api-Key "+api_key)
	}

	// Submit the request
	if !fake {
		res, err2 := client.Do(req)
		if err2 != nil {
			return err2
		}
		// Check the response
		if res.StatusCode != http.StatusOK {
			err2 = fmt.Errorf("bad status: %s", res.Status)
			return err2
		}
	}

	return nil
}

func uploadFiles(fileCh chan UploadFile, request_url string, api_key string, files_to_upload []UploadFile, wg *sync.WaitGroup) error {
	defer wg.Done()

	// Processing all links by spreading them to `free` goroutines
	for _, file := range files_to_upload {
		fileCh <- file
	}
	return nil
}

func zipAndUpload(fileCh chan UploadFile, request_url string, api_key string, files_to_zip []UploadFile, temp_dir string, wg *sync.WaitGroup) error {
	defer wg.Done()

	index := 0
	for index < len(files_to_zip) {
		zip_filename := fmt.Sprintf("upload_%d.agora_upload", index)
		zip_path := filepath.Join(temp_dir, zip_filename)
		file, err := os.Create(zip_path)
		if err != nil {
			return err
		}
		defer file.Close()

		w := zip.NewWriter(file)
		defer w.Close()

		for _, file_to_zip := range files_to_zip[index:] {
			file, err := os.Open(file_to_zip.SourcePath)
			if err != nil {
				return err
			}
			defer file.Close()

			relative_path := file_to_zip.TargetPath
			header := &zip.FileHeader{
				Name:   relative_path,
				Method: zip.Store,
			}
			f, err := w.CreateHeader(header)

			//f, err := w.Create(relative_path)
			if err != nil {
				return err
			}

			_, err = io.Copy(f, file)
			if err != nil {
				return err
			}

			index += 1

			fileInfo, err := os.Stat(zip_path)
			if err == nil && fileInfo.Size() > MAX_ZIP_SIZE {
				break
			}
		}
		w.Close()
		upload_file := UploadFile{SourcePath: zip_path, TargetPath: zip_filename, Delete: true}
		fileCh <- upload_file
	}

	return nil
}
