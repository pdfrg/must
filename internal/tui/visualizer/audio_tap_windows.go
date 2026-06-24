//go:build windows

package visualizer

func newWASAPITap() *AudioTap {
	return nil
}

func newDarwinAudioTap() *AudioTap {
	return nil
}
