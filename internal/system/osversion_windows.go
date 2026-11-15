package system

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// OSVersion es la versión de Windows, por ejemplo «Windows 10.0.26200».
func OSVersion() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("Windows %d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}
