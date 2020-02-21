package piperutils

import (
	"errors"
	"io"
	"os"
)

// IsMtaProject ...
func IsMtaProject() bool {
	mtaYaml, err := FileExists("mta.yaml")
	mtaYml, err := FileExists("mta.ym")
}
