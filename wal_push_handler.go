package walg

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"github.com/wal-g/wal-g/tracelog"
	"io/ioutil"
	"os"
	"path/filepath"
)

type CantOverwriteWalFileError struct {
	error
}

func NewCantOverwriteWalFileError(walFilePath string) CantOverwriteWalFileError {
	return CantOverwriteWalFileError{errors.Errorf("WAL file '%s' already archived, contents differ, unable to overwrite\n", walFilePath)}
}

func (err CantOverwriteWalFileError) Error() string {
	return fmt.Sprintf(tracelog.GetErrorFormatter(), err.error)
}

// TODO : unit tests
// HandleWALPush is invoked to perform wal-g wal-push
func HandleWALPush(uploader *Uploader, walFilePath string) {
	bgUploader := NewBgUploader(walFilePath, int32(getMaxUploadConcurrency(16)-1), uploader)
	// Look for new WALs while doing main upload
	bgUploader.Start()
	err := uploadWALFile(uploader, walFilePath)
	if err != nil {
		panic(err)
	}

	bgUploader.Stop()
	if uploader.deltaFileManager != nil {
		uploader.deltaFileManager.FlushFiles(uploader.Clone())
	}
} //

// TODO : unit tests
// uploadWALFile from FS to the cloud
func uploadWALFile(uploader *Uploader, walFilePath string) error {
	if uploader.uploadingFolder.preventWalOverwrite {
		overwriteAttempt, err := checkWALOverwrite(uploader, walFilePath)
		if err != nil {
			return errors.Wrap(err, "Couldn't check whether there is an overwrite attempt due to inner error")
		} else if overwriteAttempt {
			return NewCantOverwriteWalFileError(walFilePath)
		}
	}
	walFile, err := os.Open(walFilePath)
	if err != nil {
		return errors.Wrapf(err, "upload: could not open '%s'\n", walFilePath)
	}
	err = uploader.UploadWalFile(walFile)
	return errors.Wrapf(err, "upload: could not upload '%s'\n", walFilePath)
}

func checkWALOverwrite(uploader *Uploader, walFilePath string) (overwriteAttempt bool, err error) {
	walFileReader, err := downloadAndDecompressWALFile(uploader.uploadingFolder, uploader.uploadingFolder.Server+WalPath+filepath.Base(walFilePath)+"."+uploader.compressor.FileExtension())
	if err != nil {
		if _, ok := err.(ArchiveNonExistenceError); ok {
			err = nil
		}
		return false, err
	}

	archived, err := ioutil.ReadAll(walFileReader)
	if err != nil {
		return false, err
	}

	localBytes, err := ioutil.ReadFile(walFilePath)
	if err != nil {
		return false, err
	}

	if !bytes.Equal(archived, localBytes) {
		return true, nil
	} else {
		tracelog.WarningLogger.Printf("WAL file '%s' already archived, archived content equals\n", walFilePath)
		return false, nil
	}
}
