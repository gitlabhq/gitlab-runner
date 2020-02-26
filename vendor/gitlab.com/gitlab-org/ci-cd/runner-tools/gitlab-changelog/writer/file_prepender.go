package writer

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
)

func NewFilePrepender(targetFile string) Writer {
	return &filePrepender{
		buffer:     new(bytes.Buffer),
		targetFile: targetFile,
	}
}

type filePrepender struct {
	buffer     *bytes.Buffer
	targetFile string
	tempFile   string
}

func (w *filePrepender) Write(p []byte) (int, error) {
	return w.buffer.Write(p)
}

func (w *filePrepender) Flush() error {
	steps := []struct {
		description string
		call        func() error
	}{
		{
			description: "create temporary file",
			call:        w.createTempFile,
		},
		{
			description: "write buffer to the temporary file",
			call:        w.writeBufferToTemporaryFile,
		},
		{
			description: "append original data",
			call:        w.appendOriginalData,
		},
		{
			description: "replace original file",
			call:        w.replaceOriginalFile,
		},
	}

	for _, step := range steps {
		logrus.WithField("step", step.description).
			Debug("Executing filePrepender step")

		err := step.call()
		if err != nil {
			return fmt.Errorf("couldn't %s: %w", step.description, err)
		}
	}

	return nil
}

func (w *filePrepender) createTempFile() error {
	tempFile, err := ioutil.TempFile("", "changelog")
	if err != nil {
		return fmt.Errorf("couldn't create temporary file: %w", err)
	}
	_ = tempFile.Close()

	w.tempFile = tempFile.Name()

	return nil
}

func (w *filePrepender) writeBufferToTemporaryFile() error {
	target, err := os.OpenFile(w.tempFile, os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("couldn't open file %q for writing: %w", w.tempFile, err)
	}
	defer target.Close()

	logrus.WithField("target-file", w.tempFile).
		Debug("Writing buffer to file")

	_, err = w.buffer.WriteTo(target)
	if err != nil {
		return fmt.Errorf("couldn't write new content to the %q file: %w", w.tempFile, err)
	}

	return nil
}

func (w *filePrepender) appendOriginalData() error {
	_, err := os.Stat(w.targetFile)
	if errors.Is(err, os.ErrNotExist) {
		logrus.WithFields(logrus.Fields{
			"target-file": w.targetFile,
		}).Debug("File doesn't exist; skipping")

		return nil
	}

	source, err := os.Open(w.targetFile)
	if err != nil {
		return fmt.Errorf("couldn't open file %q for reading: %w", w.targetFile, err)
	}
	defer source.Close()

	target, err := os.OpenFile(w.tempFile, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		_ = source.Close()

		return fmt.Errorf("couldn't open file %q for writing: %w", w.tempFile, err)
	}
	defer target.Close()

	logrus.WithFields(logrus.Fields{
		"target-file": w.tempFile,
		"sourec-file": w.targetFile,
	}).Debug("Writing buffer to file")

	sourceReader := bufio.NewReader(source)
	_, err = sourceReader.WriteTo(target)
	if err != nil {
		return fmt.Errorf("couldn't copy data from source to target file: %w", err)
	}

	return nil
}

func (w *filePrepender) replaceOriginalFile() error {
	source, err := os.Open(w.tempFile)
	if err != nil {
		return fmt.Errorf("couldn't open file %q for reading: %w", w.tempFile, err)
	}

	target, err := os.Create(w.targetFile)
	if err != nil {
		_ = source.Close()

		return fmt.Errorf("couldn't open file %q for writing: %w", w.targetFile, err)
	}
	defer target.Close()

	logrus.WithFields(logrus.Fields{
		"target-file": w.targetFile,
		"sourec-file": w.tempFile,
	}).Debug("Copying files content")

	_, err = io.Copy(target, source)
	_ = source.Close()

	if err != nil {
		return fmt.Errorf("couldn't copy from file %q to file %q: %w", w.tempFile, w.targetFile, err)
	}

	logrus.WithField("target-file", w.tempFile).
		Debug("Removing temporary file")

	err = os.Remove(w.tempFile)
	if err != nil {
		return fmt.Errorf("couldn't remove temporary file %q: %w", w.tempFile, err)
	}

	return nil
}
