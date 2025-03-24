package models

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agoraHttp "github.com/GyroTools/gtagora-connector-go/internals/http"
	"github.com/google/uuid"
)

const (
	PARALLEL_UPLOADS        = 3
	UPLOAD_CHUCK_SIZE       = 100 * 1024 * 1024
	MAX_ZIP_SIZE            = 1024 * 1024 * 1024
	MIN_ZIP_SIZE            = 50 * 1024 * 1024
	FAKE_PROGRESS_THRESHOLD = 5 * 1024 * 1024
	ZIPPED_UPLOAD_THRESHOLD = 5
	STATE_UPLOADING         = 1
	STATE_CHECKING          = 2
	STATE_ANALYZING         = 3
	STATE_IMPORTING         = 4
	STATE_FINISHED          = 5
	STATE_ERROR             = -1

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
	Files            []UploadFile
	UploadFailed     []UploadFile
	importFinished   bool

	agoraHttp.BaseModel
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

type ImportTasks struct {
	Count    int   `json:"count"`
	Finished int   `json:"finished"`
	Error    int   `json:"error"`
	Ids      []int `json:"ids"`
}

type ImportProgress struct {
	Tasks    ImportTasks `json:"tasks"`
	State    int         `json:"state"`
	Progress int         `json:"progress"`
}

type Datafile struct {
	Id      int    `json:"state"`
	Path    string `json:"path"`
	Sha1    string `json:"sha1"`
	Dataset int    `json:"dataset"`
	Created bool   `json:"created"`
}

type ImportResult struct {
	Datafiles      []Datafile `json:"datafiles"`
	NrFiles        int
	NrUploaded     int
	NrUploadFailed int
	NrImported     int
	NrExisted      int
	NrIgnored      int
	NrHashFailed   int
	Tasks          ImportTasks
	Files          []string
	UploadFailed   []string
	Imported       []string
	Existed        []string
	Ignored        []string
	HashFailed     []string
}

type UploadFile struct {
	SourcePath  string
	TargetPath  string
	Attachments []string
	Delete      bool
	Size        int64
	isDir       bool
	Err         error
}

type ProgressType string

const (
	TypeUploadInitialized   ProgressType = "upload_initialized"
	TypeUploadStarted       ProgressType = "upload_started"
	TypeUploadCompleted     ProgressType = "upload_completed"
	TypeFileUploadStarted   ProgressType = "file_upload_started"
	TypeFileUploadCompleted ProgressType = "file_upload_completed"
	TypeFileProgress        ProgressType = "file_progress"
	TypeProgressPct         ProgressType = "progress"
	TypeMessage             ProgressType = "message"
	TypeUploadError         ProgressType = "upload_error"
	TypeImportProgress      ProgressType = "import_progress"
	TypeResultProgress      ProgressType = "result_progress"
)

type ResultProgress struct {
	NrFiles     int
	NrProcessed int
}

type UploadProgress struct {
	Type ProgressType
	Data interface{}
}

type UploadProgressInitData struct {
	FilesToZip    int
	FilesToUpload int
	TotalSize     int64
}

type UploadProgressTransferData struct {
	File            UploadFile
	TotalSize       int64
	BytesTransfered int64
	BytesIncrement  int64
	TransferRate    int64
	channel         chan UploadProgressTransferData
}

func (progressData *UploadProgressTransferData) AddBytes(bytes int64) {
	progressData.BytesTransfered += bytes
	progressData.BytesIncrement = bytes
	if progressData.channel != nil {
		progressData.channel <- *progressData
	}
}

func (progressData *UploadProgressTransferData) Complete(transferRate int64) {
	progressData.BytesTransfered = progressData.TotalSize
	progressData.BytesIncrement = 0
	progressData.TransferRate = transferRate
	if progressData.channel != nil {
		progressData.channel <- *progressData
	}
}

func (progressData *UploadProgressTransferData) Error(err error) {
	progressData.File.Err = err
	progressData.BytesIncrement = 0
	if progressData.channel != nil {
		progressData.channel <- *progressData
	}
}

func (f *UploadFile) setSize() error {
	siz := int64(0)
	fileInfo, err := os.Stat(f.SourcePath)
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		f.isDir = true
		return nil
	}
	siz += fileInfo.Size()
	for _, file := range f.Attachments {
		fileInfo, err = os.Stat(file)
		if os.IsNotExist(err) {
			return fmt.Errorf("the file \"%s\" does not exist", file)
		} else if err != nil {
			return err
		}
		siz += fileInfo.Size()
	}
	f.Size = siz
	f.isDir = false
	return nil
}

func (f *UploadFile) GetSize() int64 {
	if f.Size == 0 {
		f.setSize()
	}
	return f.Size
}

func (f *UploadFile) IsDir() bool {
	return f.isDir
}

func NewUploadFile(path string, attachments []string) (UploadFile, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return UploadFile{}, err
	}
	path = absPath
	for i, attachment := range attachments {
		absPath, err = filepath.Abs(attachment)
		if err != nil {
			return UploadFile{}, err
		}
		attachments[i] = absPath

	}
	uploadFile := UploadFile{SourcePath: path, Attachments: attachments}
	err = uploadFile.setSize()
	if err != nil {
		return uploadFile, err
	}
	if !uploadFile.IsDir() {
		uploadFile.TargetPath = filepath.Base(uploadFile.SourcePath)
	}
	return uploadFile, nil
}

func (importPackage *ImportPackage) Upload(inputFiles []UploadFile, progressChan chan UploadProgress) error {
	progressChan <- UploadProgress{Type: TypeUploadStarted, Data: importPackage.Id}
	filesToUpload, filesToZip, err := analysePaths(inputFiles)
	if err != nil {
		return err
	}
	// do not zip files if there are only a few files (ZIPPED_UPLOAD_THRESHOLD)
	if len(filesToZip) < ZIPPED_UPLOAD_THRESHOLD {
		filesToUpload = append(filesToUpload, filesToZip...)
		filesToZip = []UploadFile{}
	}
	totalSize := getTotalSize(filesToUpload, filesToZip)
	zipFilesSize := getTotalSize([]UploadFile{}, filesToZip)
	progressChan <- UploadProgress{Type: TypeUploadInitialized, Data: UploadProgressInitData{FilesToZip: len(filesToZip), FilesToUpload: len(filesToUpload), TotalSize: totalSize}}
	uploadedSize := int64(0)

	requestUrl := importPackage.Client.GetUrl(fmt.Sprintf("/api/v1/import/%d/upload/", importPackage.Id))

	// we have 2 threadpools here. One performs the large file upload and the zipping in parallel. One performs a parallel file upload
	parallelUploads := PARALLEL_UPLOADS
	fake := false

	apiKey, err := importPackage.Client.GetApiKey()
	if err != nil {
		return err
	}

	// Adding routines to workgroup and running then
	fileCh := make(chan UploadFile, parallelUploads)
	uploadBytesCh := make(chan UploadProgressTransferData, parallelUploads)
	wg := new(sync.WaitGroup)

	progressWg := new(sync.WaitGroup)
	progressWg.Add(1)
	go func() {
		defer progressWg.Done()
		for prog := range uploadBytesCh {
			if prog.File.Err != nil {
				importPackage.UploadFailed = append(importPackage.UploadFailed, prog.File)
				progressChan <- UploadProgress{Type: TypeUploadError, Data: prog.File}
			} else if prog.BytesTransfered == prog.TotalSize {
				progressChan <- UploadProgress{Type: TypeFileUploadCompleted, Data: prog.File}
			}
			progressChan <- UploadProgress{Type: TypeFileProgress, Data: prog}
			uploadedSize += prog.BytesIncrement
			progress := int(100 * uploadedSize / totalSize)
			if progress >= 100 {
				progress = 99
			}
			uploadProgress := UploadProgress{Type: TypeProgressPct, Data: progress}
			progressChan <- uploadProgress
		}
	}()

	for i := 0; i < parallelUploads; i++ {
		wg.Add(1)
		go uploadWorker(fileCh, uploadBytesCh, progressChan, requestUrl, apiKey, fake, wg)
	}

	tempDir, err := os.MkdirTemp("", "agora_interface_go")
	defer os.RemoveAll(tempDir)
	if err != nil {
		return err
	}

	wg_upload_zip := new(sync.WaitGroup)
	wg_upload_zip.Add(1)
	go uploadFiles(fileCh, filesToUpload, wg_upload_zip)
	if zipFilesSize > MAX_ZIP_SIZE {
		// if there are a lot of files to zip then we split the zipping into 3 parts and process them in parallel
		partSize := (len(filesToZip) + parallelUploads - 1) / parallelUploads
		for i := 0; i < parallelUploads; i++ {
			start := i * partSize
			end := start + partSize
			if end > len(filesToZip) {
				end = len(filesToZip)
			}
			part := filesToZip[start:end]

			wg_upload_zip.Add(1)
			go zipAndUpload(fileCh, i, part, tempDir, wg_upload_zip)
		}
	} else {
		wg_upload_zip.Add(1)
		go zipAndUpload(fileCh, 0, filesToZip, tempDir, wg_upload_zip)
	}
	wg_upload_zip.Wait()

	// Closing channel (waiting in goroutines won't continue any more)
	close(fileCh)

	// Waiting for all goroutines to finish (otherwise they die as main routine dies)
	wg.Wait()

	progressChan <- UploadProgress{Type: TypeUploadCompleted, Data: importPackage.Id}
	importPackage.Files = append(filesToUpload, filesToZip...)

	// close progress channel
	close(uploadBytesCh)
	progressWg.Wait()
	return nil
}

func (importPackage *ImportPackage) Complete(targetFolderId int, jsonImportFile string, extractZipFile bool, wg *sync.WaitGroup) error {
	path := fmt.Sprintf("/api/v1/import/%d/complete/", importPackage.Id)

	// upload the json file is exists
	if jsonImportFile != "" {
		_, err := os.Stat(jsonImportFile)
		if os.IsNotExist(err) {
			return fmt.Errorf("the json file \"%s\" does not exist", jsonImportFile)
		} else if err != nil {
			return err
		}
		requestUrl := importPackage.Client.GetUrl(fmt.Sprintf("/api/v1/import/%d/upload/", importPackage.Id))
		apiKey, err := importPackage.Client.GetApiKey()
		if err != nil {
			return err
		}
		file, err := NewUploadFile(jsonImportFile, nil)
		if err != nil {
			return err
		}
		_, err = uploadFile(nil, requestUrl, apiKey, file, false, 0)
		if err != nil {
			return err
		}
	}

	data := map[string]string{}
	if jsonImportFile != "" {
		data["import_file"] = filepath.Base(jsonImportFile)
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
		return fmt.Errorf("the \"complete\" request was invalid. http status = %d", resp.StatusCode)
	}

	if wg != nil {
		// wait for completion
		timeout := time.Duration(1) * time.Hour
		go importPackage.wait(timeout, wg)
	}

	return nil
}

func (importPackage *ImportPackage) WaitForImport(timeout time.Duration, progressChan chan UploadProgress) error {
	if importPackage.State == STATE_ERROR {
		return nil
	}

	startTime := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(timeout):
			return errors.New("import progress timeout")
		case <-ticker.C:
			curProgress, err := importPackage.progress()
			if err != nil {
				if importPackage.Client.IsTimeoutError(err) {
					continue
				}
				return err
			}
			progressChan <- UploadProgress{Type: TypeImportProgress, Data: *curProgress}
			if (curProgress.State == STATE_FINISHED || curProgress.State == STATE_ERROR) && curProgress.Progress == 100 {
				importPackage.importFinished = true
				return nil
			}
		}
		if time.Since(startTime) > timeout {
			return errors.New("import progress timeout")
		}
	}
}

func (importPackage *ImportPackage) Result(progressChan chan UploadProgress) (*ImportResult, error) {
	var result *ImportResult
	var progress *ImportProgress
	var err error
	if importPackage.importFinished {
		result, err = importPackage.result()
		if err != nil {
			return nil, err
		}
		progress, err = importPackage.progress()
		if err != nil {
			return nil, err
		}
		result.Tasks = progress.Tasks
	} else {
		result = &ImportResult{}
		result.Tasks = ImportTasks{}
	}
	result.NrFiles = len(importPackage.Files)
	for _, file := range importPackage.Files {
		result.Files = append(result.Files, file.SourcePath)
	}
	for _, file := range importPackage.UploadFailed {
		result.NrUploadFailed += 1
		result.UploadFailed = append(result.UploadFailed, file.SourcePath)
	}
	result.NrUploaded = result.NrFiles - result.NrUploadFailed

	if result.Datafiles != nil {
		// Preallocate slices with an estimated capacity
		result.Files = make([]string, 0, len(importPackage.Files))
		result.UploadFailed = make([]string, 0, len(importPackage.UploadFailed))
		result.HashFailed = make([]string, 0, len(importPackage.Files)) // Assuming worst case all files could fail hash
		result.Imported = make([]string, 0, len(importPackage.Files))   // Assuming worst case all files could be imported
		result.Existed = make([]string, 0, len(importPackage.Files))    // Assuming worst case all files could exist
		result.Ignored = make([]string, 0, len(importPackage.Files))    // Assuming worst case all files could be ignored

		step := len(importPackage.Files) / 100
		if step == 0 {
			step = 1
		}

		var resultProgress = ResultProgress{NrFiles: len(importPackage.Files), NrProcessed: 0}
		if progressChan != nil {
			progressChan <- UploadProgress{Type: TypeResultProgress, Data: resultProgress}
		}
		// Create a map from result.Datafiles for quick lookups
		datafileMap := make(map[string]Datafile)
		for _, datafile := range result.Datafiles {
			datafileMap[filepath.Clean(datafile.Path)] = datafile
		}

		for i, file := range importPackage.Files {
			cleanedTargetPath := filepath.Clean(file.TargetPath)
			datafile, found := datafileMap[cleanedTargetPath]
			if found {
				delete(datafileMap, cleanedTargetPath) // Remove the element from the map
				if datafile.Created {
					hash, err := sha1Hash(file.SourcePath)
					if err == nil {
						if hash != datafile.Sha1 {
							result.NrHashFailed += 1
							result.HashFailed = append(result.HashFailed, file.SourcePath)
						} else {
							result.NrImported += 1
							result.Imported = append(result.Imported, file.SourcePath)
						}
					} else {
						result.NrImported += 1
						result.Imported = append(result.Imported, file.SourcePath)
					}
				} else {
					result.NrExisted += 1
					result.Existed = append(result.Existed, file.SourcePath)
				}
			} else {
				result.NrIgnored += 1
				result.Ignored = append(result.Ignored, file.SourcePath)
			}
			if progressChan != nil {
				resultProgress.NrProcessed += 1
				if i%step == 0 {
					progressChan <- UploadProgress{Type: TypeResultProgress, Data: resultProgress}
				}
			}
		}
		if progressChan != nil {
			progressChan <- UploadProgress{Type: TypeResultProgress, Data: resultProgress}
		}
	}
	return result, nil
}

func (importPackage *ImportPackage) update() error {
	requestUrl := importPackage.Client.GetUrl(fmt.Sprintf("%s%d/", ImportPackageURL, importPackage.Id))
	err := importPackage.Client.GetAndParse(requestUrl, importPackage)
	if err != nil {
		return err
	}
	return nil
}

func (importPackage *ImportPackage) progress() (*ImportProgress, error) {
	requestUrl := importPackage.Client.GetUrl(fmt.Sprintf("%s%d/progress", ImportPackageURL, importPackage.Id))
	resp, err := importPackage.Client.Get(requestUrl, -1)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var curProgress ImportProgress
	err = json.NewDecoder(resp.Body).Decode(&curProgress)
	if err != nil {
		return nil, err
	}
	return &curProgress, nil
}

func (importPackage *ImportPackage) result() (*ImportResult, error) {
	requestUrl := importPackage.Client.GetUrl(fmt.Sprintf("%s%d/result", ImportPackageURL, importPackage.Id))
	resp, err := importPackage.Client.Get(requestUrl, -1)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result ImportResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, errors.New("cannot get the upload results. Please update Agora to the newest version")
	}
	return &result, nil
}

func (importPackage *ImportPackage) wait(timeout time.Duration, wg *sync.WaitGroup) error {
	defer wg.Done()
	if importPackage.State == STATE_FINISHED || importPackage.State == STATE_ERROR {
		return nil
	}

	startTime := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-time.After(timeout):
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
		if time.Since(startTime) > timeout {
			return errors.New("upload progress timeout")
		}
	}
}

func analysePaths(files []UploadFile) ([]UploadFile, []UploadFile, error) {
	var filesToUpload []UploadFile
	var filesToZip []UploadFile
	sizeZippedFiles := int64(0)
	for _, file := range files {
		if file.IsDir() {
			filepath.Walk(file.SourcePath, func(path string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					relativePath := path
					if strings.HasPrefix(path, file.SourcePath) {
						relativePath = path[len(file.SourcePath):]
					}
					relativePath = strings.Replace(relativePath, "\\", "/", -1)
					relativePath = strings.TrimPrefix(relativePath, "/")

					curUploadFile := UploadFile{SourcePath: strings.Replace(path, "\\", "/", -1), TargetPath: relativePath, Delete: false}
					curUploadFile.setSize()
					if info.Size() < UPLOAD_CHUCK_SIZE {
						filesToZip = append(filesToZip, curUploadFile)
						sizeZippedFiles += curUploadFile.GetSize()
					} else {
						filesToUpload = append(filesToUpload, curUploadFile)
					}
				}
				return nil
			})
		} else {
			if file.GetSize() < UPLOAD_CHUCK_SIZE && len(file.Attachments) == 0 {
				filesToZip = append(filesToZip, file)
			} else {
				filesToUpload = append(filesToUpload, file)
			}
		}
	}
	return filesToUpload, filesToZip, nil
}

func getTotalSize(filesToUpload []UploadFile, filesToZip []UploadFile) int64 {
	siz := int64(0)
	for _, file := range filesToZip {
		siz += file.GetSize()
		siz += int64(len(file.TargetPath))
		siz += 150 // header size (approximate)
	}

	for _, file := range filesToUpload {
		siz += file.GetSize()
	}
	return siz
}

func sha1Hash(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:]), nil
}

func sha256Hash(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:]), nil
}

func sha256HashBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func uploadWorker(fileChan chan UploadFile, uploadBytesCh chan UploadProgressTransferData, progressChan chan UploadProgress, request_url string, api_key string, fake bool, wg *sync.WaitGroup) {
	// Decreasing internal counter for wait-group as soon as goroutine finishes
	defer wg.Done()

	transferRate := int64(5 * 1024 * 1024)
	for file := range fileChan {
		progressChan <- UploadProgress{Type: TypeFileUploadStarted, Data: file}
		transferRate, _ = uploadFile(uploadBytesCh, request_url, api_key, file, fake, transferRate)
	}
}

func uploadFile(uploadBytesCh chan UploadProgressTransferData, request_url string, api_key string, file UploadFile, fake bool, transferRate int64) (int64, error) {
	fileUploadProgress := UploadProgressTransferData{File: file, BytesIncrement: 0, BytesTransfered: 0, channel: uploadBytesCh}
	buffer := make([]byte, UPLOAD_CHUCK_SIZE)
	if file.Delete {
		defer os.Remove(file.SourcePath)
	}
	mainFile := file.SourcePath
	attachmentSize := int64(0)
	totalSize := int64(0)

	fileInfo, err := os.Stat(mainFile)
	if err != nil {
		fileUploadProgress.Error(err)
		return transferRate, err
	}
	totalSize = fileInfo.Size()

	// Validate attachments and add their sizes
	for _, attachment := range file.Attachments {
		attachInfo, err := os.Stat(attachment)
		if err != nil {
			fileUploadProgress.Error(err)
			return transferRate, err
		}
		totalSize += attachInfo.Size()
		attachmentSize += attachInfo.Size()
	}

	totalChunks := int(math.Ceil(float64(fileInfo.Size()) / float64(UPLOAD_CHUCK_SIZE)))

	uuid := uuid.New()
	fileUploadProgress.TotalSize = totalSize

	// chunk number starts at 1
	curChunkNr := 1

	client := &http.Client{}
	r, err := os.Open(mainFile)
	if err != nil {
		fileUploadProgress.Error(err)
		return transferRate, err
	}
	for i := 0; i < totalChunks; i++ {
		n, err := r.Read(buffer)
		if err != nil {
			fileUploadProgress.Error(err)
			r.Close()
			return transferRate, err
		}

		if i == totalChunks-1 && len(file.Attachments) > 0 {
			lastBuffer := make([]byte, 0, int64(n)+attachmentSize)
			lastBuffer = append(lastBuffer, buffer[0:n]...)

			// Append all attachments to the last chunk
			for _, attachment := range file.Attachments {
				attachFile, err := os.Open(attachment)
				if err != nil {
					fileUploadProgress.Error(err)
					r.Close()
					return transferRate, err
				}

				attachData, err := io.ReadAll(attachFile)
				attachFile.Close()
				if err != nil {
					fileUploadProgress.Error(err)
					r.Close()
					return transferRate, err
				}

				lastBuffer = append(lastBuffer, attachData...)
			}
			buffer = lastBuffer
			n = len(buffer)
		}
		chunk := bytes.NewReader(buffer[0:n])

		chunkHash := sha256HashBytes(buffer[0:n])

		//prepare the reader instances to encode
		values := map[string]io.Reader{
			"file":                 chunk, // lets assume its this file
			"description":          strings.NewReader(""),
			"flowChunkNumber":      strings.NewReader(fmt.Sprintf("%d", curChunkNr)),
			"flowChunkSize":        strings.NewReader(fmt.Sprintf("%d", UPLOAD_CHUCK_SIZE)),
			"flowCurrentChunkSize": strings.NewReader(fmt.Sprintf("%d", n)),
			"flowTotalSize":        strings.NewReader(fmt.Sprintf("%d", totalSize)),
			"flowIdentifier":       strings.NewReader(uuid.String()),
			"flowFilename":         strings.NewReader(file.TargetPath),
			"flowRelativePath":     strings.NewReader(file.TargetPath),
			"flowTotalChunks":      strings.NewReader(fmt.Sprintf("%d", totalChunks)),
			"flowChunkHash":        strings.NewReader(chunkHash),
		}
		curChunkNr += 1

		// this goroutine sends continious (fake) progress updates since we cannot track the real number of sent bytes
		cancel := make(chan struct{})
		nrBytesSent := int64(0)
		if n > FAKE_PROGRESS_THRESHOLD {
			go func() {
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						if nrBytesSent+transferRate >= int64(n) {
							return
						}
						nrBytesSent += transferRate
						fileUploadProgress.AddBytes(transferRate)
					case <-cancel:
						// Task cancellation requested
						return
					}
				}
			}()
		}

		start := time.Now()
		err = uploadChunk(client, request_url, api_key, values, filepath.Base(file.SourcePath), fake)
		close(cancel)
		if err != nil {
			fileUploadProgress.Error(err)
			r.Close()
			return transferRate, err
		}
		duration := time.Since(start)
		fileUploadProgress.AddBytes(int64(n) - nrBytesSent)

		// calculate the transfer rate
		dur := int64(duration.Milliseconds())
		if n > FAKE_PROGRESS_THRESHOLD && dur > 0 {
			curTransferRate := 1000 * int64(n) / dur
			if i == 0 {
				transferRate = curTransferRate
			} else {
				transferRate = (curTransferRate + transferRate) / 2
			}
		}
	}
	r.Close()

	// for now remove the hash check since it needs to wait until all chunks have been joined and that might a while.
	// Also we need to poss the result from the server which is not a good design. Ideally we should check a hash for each chunk
	// but that needs a server modification
	// match, err := verifyHash(file.SourcePath, uuid.String(), api_key, request_url)
	// if err != nil {
	// 	fileUploadProgress.Error(err)
	// 	return transferRate, err
	// }
	// if !match {
	// 	err := fmt.Errorf("hashes do not match for file %s", file.SourcePath)
	// 	fileUploadProgress.Error(err)
	// 	return transferRate, err
	// }

	fileUploadProgress.Complete(transferRate)
	return transferRate, nil
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

func uploadFiles(fileCh chan UploadFile, files_to_upload []UploadFile, wg *sync.WaitGroup) error {
	defer wg.Done()

	// Processing all links by spreading them to `free` goroutines
	for _, file := range files_to_upload {
		fileCh <- file
	}
	return nil
}

func zipAndUpload(fileCh chan UploadFile, threadId int, files_to_zip []UploadFile, temp_dir string, wg *sync.WaitGroup) error {
	defer wg.Done()

	index := 0
	for index < len(files_to_zip) {
		zip_filename := fmt.Sprintf("upload_%d_%d.agora_upload", threadId, index)
		zip_path := filepath.Join(temp_dir, zip_filename)
		zipfile, err := os.Create(zip_path)
		if err != nil {
			return err
		}
		nrFilesInZip := 0

		w := zip.NewWriter(zipfile)
		for _, file_to_zip := range files_to_zip[index:] {
			file, err := os.Open(file_to_zip.SourcePath)
			if err != nil {
				return err
			}

			relative_path := file_to_zip.TargetPath
			header := &zip.FileHeader{
				Name:   relative_path,
				Method: zip.Store,
			}
			f, err := w.CreateHeader(header)
			if err != nil {
				file.Close()
				return err
			}

			_, err = io.Copy(f, file)
			if err != nil {
				file.Close()
				return err
			}
			file.Close()

			index += 1
			nrFilesInZip += 1
			fileInfo, err := os.Stat(zip_path)
			// we sent the zip file if it has exceeded its maximum size or
			// if there are free upload slots and the zip file has reached its minimum size
			if err == nil && (fileInfo.Size() > MAX_ZIP_SIZE || (fileInfo.Size() > MIN_ZIP_SIZE && len(fileCh) == 0)) {
				break
			}
		}
		w.Close()
		zipfile.Close()
		upload_file := UploadFile{SourcePath: zip_path, TargetPath: zip_filename, Delete: true}
		upload_file.setSize()
		fileCh <- upload_file
	}

	return nil
}
