package tui

import (
	"charm.land/bubbles/v2/key"
)

type KeyMap struct {
	Quit            key.Binding
	PlayPause       key.Binding
	Next            key.Binding
	Prev            key.Binding
	SeekForward     key.Binding
	SeekBackward    key.Binding
	Repeat          key.Binding
	Shuffle         key.Binding
	RestartSong     key.Binding
	CursorUp        key.Binding
	CursorDown      key.Binding
	PageUp          key.Binding
	PageDown        key.Binding
	Home            key.Binding
	End             key.Binding
	Enter           key.Binding
	Enqueue         key.Binding
	EnqueueNext     key.Binding
	CycleView       key.Binding
	Search          key.Binding
	Library         key.Binding
	Help            key.Binding
	Escape          key.Binding
	DeleteTrack     key.Binding
	ClearPlaylist   key.Binding
	Rescan          key.Binding
	Lyrics          key.Binding
	SyncedLyrics    key.Binding
	ArtistBio       key.Binding
	Gallery         key.Binding
	MoveTrackUp     key.Binding
	MoveTrackDown   key.Binding
	MoveTrackTop    key.Binding
	MoveTrackBottom key.Binding
	SavePlaylist    key.Binding
	Options         key.Binding
	SleepTimer      key.Binding
	UpdateView      key.Binding
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
	RestartSong: key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r", "restart"),
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
	EnqueueNext: key.NewBinding(
		key.WithKeys("E"),
		key.WithHelp("E", "enqueue next"),
	),
	CycleView: key.NewBinding(
		key.WithKeys("v", "tab"),
		key.WithHelp("v/tab", "switch view"),
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
		key.WithKeys("y"),
		key.WithHelp("y", "lyrics"),
	),
	SyncedLyrics: key.NewBinding(
		key.WithKeys("Y"),
		key.WithHelp("Y", "synced lyrics"),
	),
	ArtistBio: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "artist bio"),
	),
	Gallery: key.NewBinding(
		key.WithKeys("I"),
		key.WithHelp("I", "gallery"),
	),
	MoveTrackUp: key.NewBinding(
		key.WithKeys("K"),
		key.WithHelp("K", "move up"),
	),
	MoveTrackDown: key.NewBinding(
		key.WithKeys("J"),
		key.WithHelp("J", "move down"),
	),
	MoveTrackTop: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "move to top"),
	),
	MoveTrackBottom: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "move to bottom"),
	),
	SavePlaylist: key.NewBinding(
		key.WithKeys("S"),
		key.WithHelp("S", "save playlist"),
	),
	Options: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "options"),
	),
	SleepTimer: key.NewBinding(
		key.WithKeys("z"),
		key.WithHelp("z", "sleep timer"),
	),
	UpdateView: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "update view"),
	),
}

func (k KeyMap) PlaybackBindings() []key.Binding {
	return []key.Binding{
		k.PlayPause, k.Next, k.Prev,
		k.SeekForward, k.SeekBackward,
		k.Repeat, k.Shuffle, k.RestartSong,
	}
}

func (k KeyMap) NavigationBindings() []key.Binding {
	return []key.Binding{
		k.CursorUp, k.CursorDown,
		k.PageUp, k.PageDown, k.Home, k.End,
		k.Enter, k.Enqueue, k.EnqueueNext,
		k.CycleView, k.Library,
	}
}

func (k KeyMap) GlobalBindings() []key.Binding {
	return []key.Binding{
		k.Quit, k.Search, k.Help, k.Escape,
		k.Rescan, k.ClearPlaylist, k.DeleteTrack,
		k.Lyrics, k.SyncedLyrics, k.ArtistBio, k.Gallery,
		k.MoveTrackUp, k.MoveTrackDown, k.MoveTrackTop, k.MoveTrackBottom, k.SavePlaylist,
		k.Options, k.SleepTimer, k.UpdateView,
	}
}
