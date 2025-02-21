package configfile

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/denisbrodbeck/machineid"
	"github.com/sirupsen/logrus"
)

type systemIDState struct {
	systemID string
}

func newSystemIDState(filePath string) (*systemIDState, error) {
	state := &systemIDState{}

	err := state.loadFromFile(filePath)
	if err != nil {
		return nil, err
	}

	// ensure we have a system ID
	if state.GetSystemID() == "" {
		err = state.ensureSystemID()
		if err != nil {
			return nil, err
		}

		err = state.saveConfig(filePath)
		if err != nil {
			logrus.
				WithFields(logrus.Fields{
					"state_file": filePath,
					"system_id":  state.GetSystemID(),
				}).
				Warningf("Couldn't save new system ID on state file. "+
					"In order to reliably identify this runner in jobs with a known identifier,\n"+
					"please ensure there is a text file at the location specified in `state_file` "+
					"with the contents of `system_id`. Example: echo %q > %q\n", state.GetSystemID(), filePath)
		}
	}

	return state, nil
}

func (s *systemIDState) GetSystemID() string {
	return s.systemID
}

func (s *systemIDState) loadFromFile(filePath string) error {
	_, err := os.Stat(filePath)

	// permission denied is soft error
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("opening runner system ID file: %w", err)
	}

	var contents []byte
	if contents, err = os.ReadFile(filePath); err != nil {
		return fmt.Errorf("reading from runner system ID file: %w", err)
	}

	// Return a system ID only if a properly formatted value is found
	systemID := strings.TrimSpace(string(contents))
	if ok, err := regexp.MatchString("^[sr]_[0-9a-zA-Z]{12}$", systemID); err == nil && ok {
		s.systemID = systemID
	} else if err != nil {
		return fmt.Errorf("checking runner system ID: %w", err)
	}

	return nil
}

func (s *systemIDState) saveConfig(filePath string) error {
	// create directory to store configuration
	err := os.MkdirAll(filepath.Dir(filePath), 0700)
	if err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// write config file
	err = os.WriteFile(filePath, []byte(s.systemID), 0o600)
	if err != nil {
		return fmt.Errorf("writing the runner system ID: %w", err)
	}

	return nil
}

func (s *systemIDState) ensureSystemID() error {
	if s.systemID != "" {
		return nil
	}

	if systemID, err := GenerateUniqueSystemID(); err == nil {
		logrus.WithField("system_id", systemID).Info("Created missing unique system ID")

		s.systemID = systemID
	} else {
		return fmt.Errorf("generating unique system ID: %w", err)
	}

	return nil
}

func GenerateUniqueSystemID() (string, error) {
	const idLength = 12

	systemID, err := machineid.ID()
	if err == nil && systemID != "" {
		mac := hmac.New(sha256.New, []byte(systemID))
		mac.Write([]byte("gitlab-runner"))
		systemID = hex.EncodeToString(mac.Sum(nil))
		return "s_" + systemID[0:idLength], nil
	}

	// fallback to a random ID
	return generateRandomSystemID(idLength)
}

func generateRandomSystemID(idLength int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	b := make([]byte, idLength)
	max := big.NewInt(int64(len(charset)))

	for i := range b {
		r, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}

		b[i] = charset[r.Int64()]
	}
	return "r_" + string(b), nil
}
