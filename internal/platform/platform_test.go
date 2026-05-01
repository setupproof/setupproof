package platform

import "testing"

func TestNativeWindowsUnsupported(t *testing.T) {
	originalGOOS := runtimeGOOS
	defer func() {
		runtimeGOOS = originalGOOS
	}()

	runtimeGOOS = "windows"
	t.Setenv("WSL_DISTRO_NAME", "")
	if !NativeWindowsUnsupported() {
		t.Fatal("native Windows should be unsupported")
	}

	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	if NativeWindowsUnsupported() {
		t.Fatal("WSL should not be classified as native Windows")
	}

	runtimeGOOS = "linux"
	t.Setenv("WSL_DISTRO_NAME", "")
	if NativeWindowsUnsupported() {
		t.Fatal("linux should not be classified as native Windows")
	}
}
