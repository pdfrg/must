#!/bin/sh
# must launcher used by the Hyprland keybind and the desktop entry.
# Appends stderr to a state log so panic traces survive terminal closure.
# Expects the must binary on PATH (go install / make install / ~/go/bin).
exec must --layout large 2>>"${XDG_STATE_HOME:-$HOME/.local/state}/must/stderr.log"
