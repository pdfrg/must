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
	Repeat        key.Binding
	Shuffle       key.Binding
	CursorUp      key.Binding
	CursorDown    key.Binding
	PageUp        key.Binding
	PageDown      key.Binding
	Home          key.Binding
	End           key.Binding
	Enter         key.Binding
	Enqueue       key.Binding
	CycleView     key.Binding
	Search        key.Binding
	Library       key.Binding
	Help          key.Binding
	Escape        key.Binding
	DeleteTrack   key.Binding
	ClearPlaylist key.Binding
	Rescan        key.Binding
	Lyrics        key.Binding
	SyncedLyrics  key.Binding
	ArtistBio     key.Binding
	Gallery       key.Binding
}

var DefaultKeyMap = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	PlayPause: key.NewBinding(
		key.WithKeys("space"),
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
	Repeat: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "repeat"),
	),
	Shuffle: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "shuffle"),
	),
	CursorUp: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	CursorDown: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "page down"),
	),
	Home: key.NewBinding(
		key.WithKeys("home"),
		key.WithHelp("home", "jump to top"),
	),
	End: key.NewBinding(
		key.WithKeys("end"),
		key.WithHelp("end", "jump to bottom"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select/play"),
	),
	Enqueue: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "enqueue"),
	),
	CycleView: key.NewBinding(
		key.WithKeys("v", "tab"),
		key.WithHelp("v", "switch view"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Library: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "library"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back/close"),
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
	Gallery: key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("I", "gallery"),
	),
}

func (k KeyMap) PlaybackBindings() []key.Binding {
	return []key.Binding{
		k.PlayPause, k.Next, k.Prev,
		k.SeekForward, k.SeekBackward,
		k.Repeat, k.Shuffle,
	}
}

func (k KeyMap) NavigationBindings() []key.Binding {
	return []key.Binding{
		k.CursorUp, k.CursorDown,
		k.PageUp, k.PageDown, k.Home, k.End,
		k.Enter, k.Enqueue,
		k.CycleView, k.Library,
	}
}

func (k KeyMap) GlobalBindings() []key.Binding {
	return []key.Binding{
		k.Quit, k.Search, k.Help, k.Escape,
		k.Rescan, k.ClearPlaylist, k.DeleteTrack,
		k.Lyrics, k.SyncedLyrics, k.ArtistBio, k.Gallery,
	}
}
