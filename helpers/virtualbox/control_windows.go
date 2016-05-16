package virtualbox

import (
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
)

func init() {
	// Add default virtualbox location to end of local PATH
	for _, programFiles := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramW6432")} {
		if _, err := os.Stat(filepath.Join(programFiles, `Oracle\VirtualBox\VBoxManage.exe`)); err == nil {
			virtualBoxPath := filepath.Join(programFiles, `Oracle\VirtualBox`)
			path := os.Getenv("PATH")
			path += ";" + virtualBoxPath
			os.Setenv("PATH", path)
			log.Debugln("Add autodetected VirtualBoxManage.exe to end of local PATH: ", virtualBoxPath)
			break
		}
	}
}
