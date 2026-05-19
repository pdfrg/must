//go:build !windows && !darwin

package visualizer

func newWASAPITap() *AudioTap {
	return nil
}

func newDarwinAudioTap() *AudioTap {
	return nil
}
