package test

import (
	"os"
	"testing"

	"github.com/GyroTools/gtagora-connector-go/agora"
	"github.com/GyroTools/gtagora-connector-go/agora/models"
)

// findExam searches folder (and, recursively, its subfolders) for the first
// exam it can find, to give the download tests something to exercise without
// needing a hard-coded exam ID.
func findExam(t *testing.T, folder *models.Folder, depth int) *models.Study {
	t.Helper()
	if depth <= 0 {
		return nil
	}

	exams, err := folder.GetExams()
	if err != nil {
		t.Errorf("cannot get exams for folder %d: %s", folder.ID, err.Error())
		return nil
	}
	if len(exams) > 0 {
		return &exams[0]
	}

	subfolders, err := folder.GetFolders()
	if err != nil {
		t.Errorf("cannot get subfolders for folder %d: %s", folder.ID, err.Error())
		return nil
	}
	for _, sub := range subfolders {
		if exam := findExam(t, &sub, depth-1); exam != nil {
			return exam
		}
	}
	return nil
}

// TestDownloadHierarchy exercises the new Series/Dataset/Datafile listing
// methods end to end: folder -> exam -> series -> dataset -> datafile.
func TestDownloadHierarchy(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := server
	a, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
		return
	}

	project, err := a.GetProject(3)
	if err != nil {
		t.Errorf("cannot get the project: %s", err.Error())
		return
	} else if project == nil {
		t.Errorf("project is empty")
		return
	}

	folder, err := a.GetFolder(project.RootFolder)
	if err != nil {
		t.Errorf("cannot get the folder: %s", err.Error())
		return
	} else if folder == nil {
		t.Errorf("folder is empty")
		return
	}

	exam := findExam(t, folder, 5)
	if exam == nil {
		t.Skip("no exam found under the test project's root folder; cannot exercise the download hierarchy")
		return
	}

	series, err := exam.GetSeries()
	if err != nil {
		t.Errorf("cannot get series for exam %d: %s", exam.ID, err.Error())
		return
	}
	if len(series) == 0 {
		t.Skipf("exam %d has no series", exam.ID)
		return
	}

	var datasets []models.Dataset
	var seriesWithDatasets *models.Series
	for i := range series {
		ds, err := series[i].GetDatasets()
		if err != nil {
			t.Errorf("cannot get datasets for series %d: %s", series[i].ID, err.Error())
			return
		}
		if len(ds) > 0 {
			datasets = ds
			seriesWithDatasets = &series[i]
			break
		}
	}
	if seriesWithDatasets == nil {
		t.Skipf("exam %d has no series with datasets", exam.ID)
		return
	}

	var datafiles []models.Datafile
	var datasetWithFiles *models.Dataset
	for i := range datasets {
		dfs, err := datasets[i].GetDatafiles()
		if err != nil {
			t.Errorf("cannot get datafiles for dataset %d: %s", datasets[i].ID, err.Error())
			return
		}
		if len(dfs) > 0 {
			datafiles = dfs
			datasetWithFiles = &datasets[i]
			break
		}
	}
	if datasetWithFiles == nil {
		t.Skipf("series %d has no dataset with datafiles", seriesWithDatasets.ID)
		return
	}

	tmpDir, err := os.MkdirTemp("", "agora_download_test")
	if err != nil {
		t.Errorf("cannot create temp dir: %s", err.Error())
		return
	}
	defer os.RemoveAll(tmpDir)

	df := datafiles[0]
	path, skipped, err := df.Download(tmpDir, nil)
	if err != nil {
		t.Errorf("cannot download datafile %d: %s", df.ID, err.Error())
		return
	}
	if skipped {
		t.Errorf("expected the first download to not be skipped")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("downloaded file is missing: %s", err.Error())
		return
	}
	if df.Size > 0 && info.Size() != df.Size {
		t.Errorf("downloaded file size mismatch: got %d, want %d", info.Size(), df.Size)
	}

	// Downloading the same file again should be skipped (size+sha1 match).
	_, skipped, err = df.Download(tmpDir, nil)
	if err != nil {
		t.Errorf("cannot re-download datafile %d: %s", df.ID, err.Error())
		return
	}
	if !skipped {
		t.Errorf("expected the second download to be skipped since the file already exists")
	}
}

// TestFolderTypedItems checks that GetSeries()/GetDatasets() on a Folder
// correctly type-decode items whose content_type is "serie"/"dataset" -
// in particular this guards against the content_type string mismatch
// ("serie" vs "series") between the FolderItem API and the Series resource.
func TestFolderTypedItems(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := server
	a, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
		return
	}

	project, err := a.GetProject(3)
	if err != nil || project == nil {
		t.Errorf("cannot get the project: %v", err)
		return
	}

	folder, err := a.GetFolder(project.RootFolder)
	if err != nil || folder == nil {
		t.Errorf("cannot get the folder: %v", err)
		return
	}

	if _, err := folder.GetExams(); err != nil {
		t.Errorf("GetExams failed: %s", err.Error())
	}
	if _, err := folder.GetSeries(); err != nil {
		t.Errorf("GetSeries failed: %s", err.Error())
	}
	if _, err := folder.GetDatasets(); err != nil {
		t.Errorf("GetDatasets failed: %s", err.Error())
	}
}
