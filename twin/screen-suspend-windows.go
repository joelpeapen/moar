//go:build windows

package twin

import "fmt"

func (screen *UnixScreen) suspend() error {
	return fmt.Errorf("suspend is not supported on windows")
}
