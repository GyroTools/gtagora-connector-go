package test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/GyroTools/gtagora-connector-go/agora"
	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

const server = "https://chap02.ethz.ch"

func createTempDirectory() (string, error) {
	dir, err := ioutil.TempDir("", "example")
	if err != nil {
		return "", err
	}

	// Create files in the temporary directory
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")

	err = ioutil.WriteFile(file1, []byte("File 1 content"), 0644)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(file2, []byte("File 2 content"), 0644)
	if err != nil {
		return "", err
	}

	return dir, nil
}

func TestFolder(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := server
	agora, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	project, err := agora.GetProject(3)
	if err != nil {
		t.Errorf("cannot get the project: %s", err.Error())
	} else if project == nil {
		t.Errorf("project is empty")
	}

	folder, err := agora.GetFolder(*project.RootFolder)
	if err != nil {
		t.Errorf("cannot get the folder: %s", err.Error())
	} else if folder == nil {
		t.Errorf("folder is empty")
	}

	items, err := folder.GetItems()
	if err != nil {
		t.Errorf("cannot get the items: %s", err.Error())
	} else if len(items) == 0 {
		t.Errorf("items is empty")
	}

	folders, err := folder.GetFolders()
	if err != nil {
		t.Errorf("cannot get the subfolders: %s", err.Error())
	} else if len(folders) == 0 {
		t.Errorf("subfolders is empty")
	}
}

func TestPing(t *testing.T) {
	url := server
	err := agora.Ping(url)
	if err != nil {
		t.Errorf("could not ping Agora: %s", err.Error())
	}

	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	client := http.NewClient(url, apiKey, false)
	err = client.Ping()
	if err != nil {
		t.Errorf("could not ping Agora: %s", err.Error())
	}
}

func TestConnect(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := server
	_, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "https://chap02.ethz.ch/api/v2/project/"
	_, err = agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "chap02.ethz.ch"
	_, err = agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "chap02.ethz.ch/api/v2/project/"
	_, err = agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}
}

func TestConnectWithPassword(t *testing.T) {
	username := os.Getenv("AGORA_USERNAME")
	if len(username) == 0 {
		t.Errorf("did not find an username in the environment variable AGORA_USERNAME")
		return
	}
	password := os.Getenv("AGORA_PASSWORD")
	if len(password) == 0 {
		t.Errorf("did not find an username in the environment variable AGORA_PASSWORD")
		return
	}

	url := server
	_, err := agora.CreateWithPassword(url, username, password, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "https://chap02.ethz.ch/api/v2/project/"
	_, err = agora.CreateWithPassword(url, username, password, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "chap02.ethz.ch"
	_, err = agora.CreateWithPassword(url, username, password, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	url = "chap02.ethz.ch/api/v2/project/"
	_, err = agora.CreateWithPassword(url, username, password, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}
}

func TestGetApiKey(t *testing.T) {
	username := os.Getenv("AGORA_USERNAME")
	if len(username) == 0 {
		t.Errorf("did not find an username in the environment variable AGORA_USERNAME")
		return
	}
	password := os.Getenv("AGORA_PASSWORD")
	if len(password) == 0 {
		t.Errorf("did not find an username in the environment variable AGORA_PASSWORD")
		return
	}
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := server
	passwordClient := http.NewPasswordClient(url, username, password, false)
	apiKey, err := passwordClient.GetApiKey()
	if err != nil {
		t.Errorf("could not get the api key: %s", err.Error())
	} else if len(apiKey) == 0 {
		t.Errorf("api key is empty")
	}

	apiKeyClient := http.NewClient(url, apiKey, false)
	apiKey2, err := apiKeyClient.GetApiKey()
	if err != nil {
		t.Errorf("could not get the api key: %s", err.Error())
	} else if len(apiKey2) == 0 {
		t.Errorf("api key is empty")
	} else if apiKey != apiKey2 {
		t.Errorf("api keys are different")
	}

	agorak, err := agora.Create(url, apiKey, false)
	apiKey3, err := agorak.GetApiKey()
	if err != nil {
		t.Errorf("could not get the api key: %s", err.Error())
	} else if len(apiKey3) == 0 {
		t.Errorf("api key is empty")
	} else if apiKey != apiKey3 {
		t.Errorf("api keys are different")
	}

	agorap, err := agora.CreateWithPassword(url, username, password, false)
	apiKey4, err := agorap.GetApiKey()
	if err != nil {
		t.Errorf("could not get the api key: %s", err.Error())
	} else if len(apiKey4) == 0 {
		t.Errorf("api key is empty")
	} else if apiKey != apiKey4 {
		t.Errorf("api keys are different")
	}
}

func TestGetProjects(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := server
	agora, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	project, err := agora.GetProject(3)
	if err != nil {
		t.Errorf("cannot get the project: %s", err.Error())
	} else if project == nil {
		t.Errorf("project is empty")
	}

	projects, err := agora.GetProjects()
	if err != nil {
		t.Errorf("cannot get the projects: %s", err.Error())
	} else if len(projects) == 0 {
		t.Errorf("projects is empty")
	}
}

func TestFolderItem(t *testing.T) {
	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := server
	agora, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
	}

	item, err := agora.GetFolderItem(392)
	if err != nil {
		t.Errorf("cannot get the folder item: %s", err.Error())
	} else if item == nil {
		t.Errorf("folder item is empty")
	}
}

func TestImportPackage(t *testing.T) {
	tempDir, err := createTempDirectory()
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	apiKey := os.Getenv("AGORA_API_KEY")
	if len(apiKey) == 0 {
		t.Errorf("did not find an api key in the environment variable AGORA_API_KEY")
		return
	}

	url := server
	agora, err := agora.Create(url, apiKey, false)
	if err != nil {
		t.Errorf("could not connect to Agora: %s", err.Error())
		return
	}

	importPackage, err := agora.NewImportPackage()
	if err != nil {
		t.Errorf("cannot get the import package: %s", err.Error())
	} else if importPackage == nil {
		t.Errorf("import package is empty")
		return
	}

	files := []string{filepath.Join(tempDir, "file1.txt"), filepath.Join(tempDir, "file2.txt")}
	err = importPackage.Upload(files)
	if err != nil {
		t.Errorf("cannot upload files: %s", err.Error())
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)
	err = importPackage.Complete(123, "", false, &wg)
	if err != nil {
		t.Errorf("cannot send the complete request: %s", err.Error())
		return
	}
	wg.Wait()
}
