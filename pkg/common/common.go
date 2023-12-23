package common

import "os"

const (
	ConfigMaps = "configmaps"
	Secrets    = "secrets"
)

func GetWd() string {
	wd := os.Getenv("WORK_DIR")
	if wd == "" {
		wd, _ = os.Getwd()
	}
	return wd
}
