package tui

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Quit          key.Binding
	PlayPause     key.Binding
	Next          key.Binding
	Prev          key.Binding
	SeekForward   key.Binding
	SeekBackward  key.Binding
	VolumeUp      key.Binding
	VolumeDown    key.Binding
	Mute          key.Binding
	Repeat        key.Binding
	Shuffle       key.Binding
	CursorUp      key.Binding
	CursorDown    key.Binding
	Enter         key.Binding
	FocusLeft     key.Binding
	FocusRight    key.Binding
	CycleView     key.Binding
	Search        key.Binding
	Help          key.Binding
	Escape        key.Binding
	DeleteTrack   key.Binding
	ClearPlaylist key.Binding
	Rescan        key.Binding
	Lyrics        key.Binding
	SyncedLyrics  key.Binding
	ArtistBio     key.Binding
}

var DefaultKeyMap = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	PlayPause: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "play/pause"),
	),
	Next: key.NewBinding(
		key.WithKeys("n"),
		key.WithHelp("n", "next"),
	),
	Prev: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "prev"),
	),
	SeekForward: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "+5s"),
	),
	SeekBackward: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "-5s"),
	),
	VolumeUp: key.NewBinding(
		key.WithKeys("up", "+"),
		key.WithHelp("↑/+", "vol up"),
	),
	VolumeDown: key.NewBinding(
		key.WithKeys("down", "-"),
		key.WithHelp("↓/-", "vol down"),
	),
	Mute: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "mute"),
	),
	Repeat: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "repeat"),
	),
	Shuffle: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "shuffle"),
	),
	CursorUp: key.NewBinding(
		key.WithKeys("k"),
		key.WithHelp("k", "up"),
	),
	CursorDown: key.NewBinding(
		key.WithKeys("j"),
		key.WithHelp("j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	FocusLeft: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "focus left"),
	),
	FocusRight: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "focus right"),
	),
	CycleView: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch view"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	DeleteTrack: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete track"),
	),
	ClearPlaylist: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "clear playlist"),
	),
	Rescan: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "rescan library"),
	),
	Lyrics: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "lyrics"),
	),
	SyncedLyrics: key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "synced lyrics"),
	),
	ArtistBio: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "artist bio"),
	),
}

func (k KeyMap) PlaybackBindings() []key.Binding {
	return []key.Binding{
		k.PlayPause, k.Next, k.Prev,
		k.SeekForward, k.SeekBackward,
		k.VolumeUp, k.VolumeDown, k.Mute,
		k.Repeat, k.Shuffle,
	}
}

func (k KeyMap) NavigationBindings() []key.Binding {
	return []key.Binding{
		k.CursorUp, k.CursorDown,
		k.Enter, k.FocusLeft, k.FocusRight,
		k.CycleView,
	}
}

func (k KeyMap) GlobalBindings() []key.Binding {
	return []key.Binding{
		k.Quit, k.Search, k.Help, k.Escape,
		k.Rescan, k.ClearPlaylist, k.DeleteTrack,
		k.Lyrics, k.SyncedLyrics, k.ArtistBio,
	}
}
