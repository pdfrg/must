package main

import (
	"fmt"
	"github.com/pdfrg/must/internal/api"
)

func main() {
	// Test with single-word artist (no URL encoding needed)
	a1, e1 := api.SearchArtistTheAudioDB("", "Radiohead", "OK Computer")
	fmt.Printf("Radiohead: err=%v, bio=%d, thumb=%v, fanarts=%d\n",
		e1, len(a1.StrBiography), a1.StrArtistThumb != "", len(a1.FanArts()))

	// Test with multi-word artist (needs URL encoding)
	a2, e2 := api.SearchArtistTheAudioDB("", "Depeche Mode", "")
	fmt.Printf("Depeche Mode (no album): err=%v, bio=%d, thumb=%v, fanarts=%d\n",
		e2, len(a2.StrBiography), a2.StrArtistThumb != "", len(a2.FanArts()))

	// Test Discogs bio cleaning
	d3, e3 := api.SearchArtistDiscogs("", "", "", "Depeche Mode")
	fmt.Printf("Discogs cleaned: err=%v, profile_len=%d\n", e3, len(d3.Profile))
	fmt.Printf("  Preview: %.200s\n", d3.Profile)
}
