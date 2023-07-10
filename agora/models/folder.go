package models

import (
	"encoding/json"
	"fmt"

	"github.com/GyroTools/gtagora-connector-go/internals/http"
)

const FolderURL = "api/v2/folder/"
const FolderItemURL = "api/v2/folderitem/"
const (
	ContentTypeFolder  = "folder"
	ContentTypeExam    = "exam"
	ContentTypeSeries  = "series"
	ContentTypeDataset = "dataset"
)

type Folder struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Project *int   `json:"project"`

	http.BaseModel
}

type FolderItem struct {
	ContentObject ContentObject `json:"content_object"`
	Folder        int           `json:"folder"`
	ContentType   string        `json:"content_type"`
	ObjectID      int           `json:"object_id"`
	ID            int           `json:"id"`
	ModifiedDate  string        `json:"modified_date"`
	IsLink        bool          `json:"is_link"`

	http.BaseModel
}

type ContentObject interface{}

type FolderContent struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Project int    `json:"project"`
}

func (folder *Folder) GetItems() ([]FolderItem, error) {
	path := fmt.Sprintf("%s%d/items/", FolderURL, folder.ID)
	path += "?limit=10000000000"
	var folderItems []FolderItem
	err := folder.Client.GetAndParse(path, &folderItems)
	if err != nil {
		return nil, err
	}
	return folderItems, nil
}

func (folder *Folder) GetFolders() ([]Folder, error) {
	items, err := folder.GetItems()
	if err != nil {
		return nil, err
	}
	var folders []Folder
	for _, item := range items {
		if item.ContentType == ContentTypeFolder {
			contentBytes, err := json.Marshal(item.ContentObject)
			if err != nil {
				return nil, err
			}
			var curFolder Folder
			err = json.Unmarshal(contentBytes, &curFolder)
			if err != nil {
				return nil, err
			}
			curFolder.URL = item.URL
			curFolder.Client = item.Client
			folders = append(folders, curFolder)
		}
	}
	return folders, nil
}
