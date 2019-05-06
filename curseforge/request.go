package curseforge
import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// addonSlugRequest is sent to the CurseProxy GraphQL api to get the id from a slug
type addonSlugRequest struct {
	Query     string `json:"query"`
	Variables struct {
		Slug string `json:"slug"`
	} `json:"variables"`
}

// addonSlugResponse is received from the CurseProxy GraphQL api to get the id from a slug
type addonSlugResponse struct {
	Data struct {
		Addons []struct {
			ID int `json:"id"`
		} `json:"addons"`
	} `json:"data"`
	Exception  string   `json:"exception"`
	Message    string   `json:"message"`
	Stacktrace []string `json:"stacktrace"`
}

// Most of this is shamelessly copied from my previous attempt at modpack management:
// https://github.com/comp500/modpack-editor/blob/master/query.go
func modIDFromSlug(slug string) (int, error) {
	request := addonSlugRequest{
		Query: `
		query getIDFromSlug($slug: String) {
			{
				addons(slug: $slug) {
					id
				}
			}
		}
		`,
	}
	request.Variables.Slug = slug

	// Uses the curse.nikky.moe GraphQL api
	var response addonSlugResponse
	client := &http.Client{}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", "https://curse.nikky.moe/graphql", bytes.NewBuffer(requestBytes))
	if err != nil {
		return 0, err
	}

	// TODO: make this configurable application-wide
	req.Header.Set("User-Agent", "comp500/packwiz client")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		return 0, err
	}

	if len(response.Exception) > 0 || len(response.Message) > 0 {
		return 0, fmt.Errorf("Error requesting id for slug: %s", response.Message)
	}

	if len(response.Data.Addons) < 1 {
		return 0, errors.New("Addon not found")
	}

	return response.Data.Addons[0].ID, nil
}

// curseMetaError is an error returned by the Staging CurseMeta API
type curseMetaError struct {
	Description string `json:"description"`
	Error       bool   `json:"error"`
	Status      int    `json:"status"`
}

const (
	fileTypeRelease int = iota + 1
	fileTypeBeta
	fileTypeAlpha
)

const (
	dependencyTypeRequired int = iota + 1
	dependencyTypeOptional
)

// modInfo is a subset of the deserialised JSON response from the Staging CurseMeta API for mods (addons)
type modInfo struct {
	Name                   string    `json:"name"`
	Slug                   string    `json:"slug"`
	ID                     int       `json:"id"`
	LatestFiles            []modFile `json:"latestFiles"`
	GameVersionLatestFiles []struct {
		// TODO: check how twitch launcher chooses which one to use, when you are on beta/alpha channel?!
		// or does it not have the concept of channels?!
		GameVersion string `json:"gameVersion"`
		ID          int    `json:"projectFileId"`
		Name        string `json:"projectFileName"`
		FileType    int    `json:"fileType"`
	} `json:"gameVersionLatestFiles"`
}

func getModInfo(modid int) (modInfo, error) {
	// Uses the Staging CurseMeta api
	var response struct {
		modInfo
		curseMetaError
	}
	client := &http.Client{}

	idStr := strconv.Itoa(modid)

	req, err := http.NewRequest("GET", "https://staging_cursemeta.dries007.net/api/v3/direct/addon/"+idStr, nil)
	if err != nil {
		return modInfo{}, err
	}

	// TODO: make this configurable application-wide
	req.Header.Set("User-Agent", "comp500/packwiz client")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return modInfo{}, err
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil && err != io.EOF {
		return modInfo{}, err
	}

	if response.Error {
		return modInfo{}, fmt.Errorf("Error requesting mod metadata: %s", response.Description)
	}

	if response.ID != modid {
		return modInfo{}, fmt.Errorf("Unexpected addon ID in CurseForge response: %d/%d", modid, response.ID)
	}

	return response.modInfo, nil
}

type modFile struct {
	ID           int       `json:"id"`
	FileName     string    `json:"fileNameOnDisk"`
	FriendlyName string    `json:"fileName"`
	Date         time.Time `json:"fileDate"`
	Length       int       `json:"fileLength"`
	FileType     int       `json:"releaseType"`
	// fileStatus? means latest/preferred?
	GameVersions []string `json:"gameVersion"`
	Dependencies []struct {
		ModID int `json:"addonId"`
		Type  int `json:"type"`
	} `json:"dependencies"`
}

