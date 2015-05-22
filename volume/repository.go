package volume

import (
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/lager"
)

type Strategy map[string]string

type Volume struct {
	GUID string `json:"guid"`
	Path string `json:"path"`
}

type Volumes []Volume

var ErrMissingStrategy = errors.New("missing strategy")
var ErrUnrecognizedStrategy = errors.New("unrecognized strategy")
var ErrCreateVolumeFailed = errors.New("failed to create volume")
var ErrListVolumesFailed = errors.New("failed to list volumes")
var ErrNoParentVolumeProvided = errors.New("no parent volume provided")
var ErrParentVolumeNotFound = errors.New("parent volume not found")

type Repository struct {
	volumeDir string

	logger lager.Logger
}

func NewRepository(logger lager.Logger, volumeDir string) *Repository {
	return &Repository{
		volumeDir: volumeDir,
		logger:    logger,
	}
}

func (repo *Repository) CreateVolume(strategy Strategy) (Volume, error) {
	guid := repo.createUuid()
	createdVolume := filepath.Join(repo.volumeDir, guid)

	strategyName, found := strategy["type"]
	if !found {
		return Volume{}, ErrMissingStrategy
	}

	logger := repo.logger.Session("create-volume", lager.Data{
		"strategy": strategyName,
	})

	if strategyName == "empty" {
		err := os.MkdirAll(createdVolume, 0755)
		if err != nil {
			repo.logger.Error("failed-to-make-dir", err, lager.Data{
				"volume-path": createdVolume,
			})

			return Volume{}, ErrCreateVolumeFailed
		}
	} else if strategyName == "cow" {
		parentVolume, found := strategy["volume"]
		if !found {
			logger.Error("no-parent-volume-provided", nil)

			return Volume{}, ErrNoParentVolumeProvided
		}

		if !repo.volumeExists(parentVolume) {
			logger.Error("parent-volume-not-found", nil)

			return Volume{}, ErrParentVolumeNotFound
		}

		err := exec.Command("cp", "-r", filepath.Join(repo.volumeDir, parentVolume), createdVolume).Run()

		if err != nil {
			logger.Error("failed-to-copy-volume", err)

			return Volume{}, ErrCreateVolumeFailed
		}
	} else {
		logger.Error("unrecognized-strategy", nil, lager.Data{
			"strategy": strategyName,
		})

		return Volume{}, ErrUnrecognizedStrategy
	}

	return Volume{
		Path: createdVolume,
		GUID: guid,
	}, nil
}

func (repo *Repository) ListVolumes() (Volumes, error) {
	volumes, err := ioutil.ReadDir(repo.volumeDir)
	if err != nil {
		repo.logger.Error("failed-to-list-dirs", err, lager.Data{
			"volume-dir": repo.volumeDir,
		})

		return Volumes{}, ErrListVolumesFailed
	}

	response := make(Volumes, 0, len(volumes))
	for _, volume := range volumes {
		response = append(response, Volume{
			GUID: volume.Name(),
			Path: filepath.Join(repo.volumeDir, volume.Name()),
		})
	}

	return response, nil
}

func (repo *Repository) createUuid() string {
	guid, err := uuid.NewV4()
	if err != nil {
		repo.logger.Fatal("failed-to-generate-guid", err)
	}

	return guid.String()
}

func (repo *Repository) volumeExists(guid string) bool {
	info, err := os.Stat(filepath.Join(repo.volumeDir, guid))
	if err != nil {
		return false
	}

	return info.IsDir()
}
