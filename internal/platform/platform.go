package platform

import (
	"os"
	"runtime"
)

const NativeWindowsUnsupportedMessage = "native Windows execution is unsupported in v0.1; use WSL2"

var runtimeGOOS = runtime.GOOS
var runtimeGOARCH = runtime.GOARCH

func NativeWindowsUnsupported() bool {
	return runtimeGOOS == "windows" && os.Getenv("WSL_DISTRO_NAME") == ""
}

func GOOS() string {
	return runtimeGOOS
}

func GOARCH() string {
	return runtimeGOARCH
}
