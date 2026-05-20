package genre

import (
	"regexp"
	"strconv"
	"strings"
)

var id3v2Genres = [...]string{
	"Blues", "Classic Rock", "Country", "Dance", "Disco", "Funk", "Grunge",
	"Hip-Hop", "Jazz", "Metal", "New Age", "Oldies", "Other", "Pop", "R&B",
	"Rap", "Reggae", "Rock", "Techno", "Industrial", "Alternative", "Ska",
	"Death Metal", "Pranks", "Soundtrack", "Euro-Techno", "Ambient",
	"Trip-Hop", "Vocal", "Jazz+Funk", "Fusion", "Trance", "Classical",
	"Instrumental", "Acid", "House", "Game", "Sound Clip", "Gospel",
	"Noise", "AlternRock", "Bass", "Soul", "Punk", "Space", "Meditative",
	"Instrumental Pop", "Instrumental Rock", "Ethnic", "Gothic",
	"Darkwave", "Techno-Industrial", "Electronic", "Pop-Folk",
	"Eurodance", "Dream", "Southern Rock", "Comedy", "Cult", "Gangsta",
	"Top 40", "Christian Rap", "Pop/Funk", "Jungle", "Native American",
	"Cabaret", "New Wave", "Psychedelic", "Rave", "Showtunes", "Trailer",
	"Lo-Fi", "Tribal", "Acid Punk", "Acid Jazz", "Polka", "Retro",
	"Musical", "Rock & Roll", "Hard Rock", "Folk", "Folk-Rock",
	"National Folk", "Swing", "Fast Fusion", "Bebob", "Latin", "Revival",
	"Celtic", "Bluegrass", "Avantgarde", "Gothic Rock", "Progressive Rock",
	"Psychedelic Rock", "Symphonic Rock", "Slow Rock", "Big Band",
	"Chorus", "Easy Listening", "Acoustic", "Humour", "Speech", "Chanson",
	"Opera", "Chamber Music", "Sonata", "Symphony", "Booty Bass", "Primus",
	"Porn Groove", "Satire", "Slow Jam", "Club", "Tango", "Samba",
	"Folklore", "Ballad", "Power Ballad", "Rhythmic Soul", "Freestyle",
	"Duet", "Punk Rock", "Drum Solo", "A capella", "Euro-House", "Dance Hall",
	"Goa", "Drum & Bass", "Club-House", "Hardcore", "Terror", "Indie",
	"Britpop", "Negerpunk", "Polsk Punk", "Beat", "Christian Gangsta Rap",
	"Heavy Metal", "Black Metal", "Crossover", "Contemporary Christian",
	"Christian Rock ", "Merengue", "Salsa", "Thrash Metal", "Anime", "JPop",
	"Synthpop",
	"Christmas", "Art Rock", "Baroque", "Bhangra", "Big Beat", "Breakbeat",
	"Chillout", "Downtempo", "Dub", "EBM", "Eclectic", "Electro",
	"Electroclash", "Emo", "Experimental", "Garage", "Global", "IDM",
	"Illbient", "Industro-Goth", "Jam Band", "Krautrock", "Leftfield", "Lounge",
	"Math Rock", "New Romantic", "Nu-Breakz", "Post-Punk", "Post-Rock", "Psytrance",
	"Shoegaze", "Space Rock", "Trop Rock", "World Music", "Neoclassical", "Audiobook",
	"Audio Theatre", "Neue Deutsche Welle", "Podcast", "Indie Rock", "G-Funk", "Dubstep",
	"Garage Rock", "Psybient",
}

var parenRe = regexp.MustCompile(`\((\d+)\)`)

func resolvePart(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if idx, err := strconv.Atoi(p); err == nil && idx >= 0 && idx < len(id3v2Genres) {
		return id3v2Genres[idx]
	}
	if m := parenRe.FindStringSubmatch(p); len(m) > 1 {
		if idx, err := strconv.Atoi(m[1]); err == nil && idx >= 0 && idx < len(id3v2Genres) {
			return id3v2Genres[idx]
		}
	}
	return p
}

func Normalize(genre string) string {
	if genre == "" {
		return ""
	}
	var parts []string
	for _, sep := range []string{";", "/", "\x00"} {
		if strings.Contains(genre, sep) {
			parts = strings.Split(genre, sep)
			break
		}
	}
	if parts == nil {
		parts = []string{genre}
	}
	seen := make(map[string]bool)
	var resolved []string
	for _, p := range parts {
		r := resolvePart(p)
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		low := strings.ToLower(r)
		if seen[low] {
			continue
		}
		seen[low] = true
		resolved = append(resolved, r)
	}
	return strings.Join(resolved, ";")
}

func Split(genre string) []string {
	normalized := Normalize(genre)
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, ";")
}
