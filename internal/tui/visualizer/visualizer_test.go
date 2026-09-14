package visualizer

import "testing"

func TestSyntheticVisualizerIsImmediatelyReady(t *testing.T) {
	v := New(1)
	if !v.AudioReady() {
		t.Fatal("synthetic visualizer should not wait for an external audio source")
	}
}
