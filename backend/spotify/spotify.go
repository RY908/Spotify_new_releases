package spotify

import (
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
	"time"
	"fmt"
	"os"
	"strings"
	. "Spotify_new_releases/database"
)

// GetCurrentUserId returns user id of the current client.
func (c *Client) GetCurrentUserId() (string, error) {
	// get a current user
	user, err := c.Client.CurrentUser()
	if err != nil {
		return "", err
	}
	return user.ID, nil
}

// CreatePlaylistForUser makes a new spotify playlist for a user.
func (c *Client) CreatePlaylistForUser(userId string) (*spotify.FullPlaylist, error){
	// create a new spotify playlist
	playlist, err := c.Client.CreatePlaylistForUser(userId, "New Releases", "", true)
	if err != nil {
		return nil, err
	}
	return playlist, err
}

// GetRecentlyPlayedArtists returns a list of artists who the user played recently.
func (c *Client) GetRecentlyPlayedArtists() (map[spotify.ID]spotify.FullArtist, map[string]int, *oauth2.Token) {
	// get tracks which are played within 50 minutes 
	t := time.Now().UTC() // present time
	t = t.Add(-20 * time.Minute) // 50 minutes before present time 
	afterTime := t.UnixNano() / int64(time.Millisecond) // convert to milliseconds
	recentlyPlayedItems, _ := c.Client.PlayerRecentlyPlayedOpt(&spotify.RecentlyPlayedOptions{Limit: 50, AfterEpochMs: afterTime}) // get recentlyPlayedItems

	artistsSet := make(map[spotify.ID]spotify.FullArtist) // set of artists
	artistsCount := make(map[string]int) // counter
	
	// add an artist to artistsSet if the artist is not existed in artistsSet
	for _, item := range recentlyPlayedItems {
		for _, artist := range item.Track.Artists {
			if _, ok := artistsSet[artist.ID]; !ok {
				fullArtist, _ := c.Client.GetArtist(spotify.ID(artist.ID))
				artistsSet[artist.ID] = *fullArtist
				artistsCount[string(artist.ID)] = 0
			} else {
				artistsCount[string(artist.ID)] += 1
			}
		}
	}

	// get new token
	token, _ := c.Client.Token()

	return artistsSet, artistsCount, token

}

// GetNewReleases returns new releases based on the user's interests.
func (c *Client) GetNewReleases(artists []ArtistInfo, userId string) ([]spotify.SimpleAlbum, error) {
	var newReleases []spotify.SimpleAlbum

	// present UTC time
	now := time.Now().UTC()

	// 7 days from present time
	after := now.AddDate(0, 0, -7)

	// get current user information
	user, _ := c.Client.CurrentUser()

	for _, artist := range artists {
		artistId := artist.ArtistId // artist id to search
		offset := 0
		limit := 10 
		
		opt := spotify.Options{Country:&user.Country, Limit:&limit, Offset:&offset}
		albums, err := c.Client.GetArtistAlbumsOpt(spotify.ID(artistId), &opt, 2) // get albums
		if err != nil {
			return nil, err
		}
		// if the album or single is released this week, add the track to newReleases
		for _, album := range albums.Albums {
			if album.ReleaseDateTime().After(after) {
				newReleases = append(newReleases, album)
				//fmt.Println(album.Name)
			}
		}
		time.Sleep(time.Millisecond * 500)

	}
	return newReleases, nil
}

// GetFollowingArtists returns artists' information a user follows.
func (c *Client) GetFollowingArtists(userId string) ([]ArtistInfo, error) {
	lastId := ""
	var artists []ArtistInfo

	// By specifying lastId, CurrentUsersFollowedArtistsOpt returns the next 50 artists the user follows.
	for {
		following, err := c.Client.CurrentUsersFollowedArtistsOpt(50, lastId)
		if err != nil {
			return nil, err
		}
		for _, following := range following.Artists {
			var name, artistId, url, iconUrl string
			name, artistId, url, iconUrl = GetArtistInfo(following)
			lastId = artistId
			artists = append(artists, ArtistInfo{ArtistId: artistId, Name:name, Url:url, IconUrl:iconUrl})
		}

		if len(following.Artists) < 50 {
			break
		}
	}
	return artists, nil
}

func (c *Client) SetConfig(playlistId spotify.ID) error {
	img, _ := os.Open("./img/logo.png")
	description := "playlist made by https://newreleases.tk"
	if err := c.Client.SetPlaylistImage(playlistId, img); err != nil {
		return err
	}
	if err := c.Client.ChangePlaylistDescription(playlistId, description); err != nil {
		return err
	}
	return nil

}
 
// GetArtistInfo retrieves artist's name, id, url, iconUrl and returns them.
func GetArtistInfo(artist spotify.FullArtist) (string, string, string, string) {
	var name, artistId, url, iconUrl string
	name = artist.SimpleArtist.Name
	artistId = artist.SimpleArtist.ID.String()
	url = artist.SimpleArtist.ExternalURLs["spotify"]
	if len(artist.Images) > 0 {
		iconUrl = artist.Images[0].URL
	} else {
		iconUrl = ""
	}

	return name, artistId, url, iconUrl
}

// ChangePlaylist change tracks in the playlist "new releases".
func ChangePlaylist(newReleases []spotify.SimpleAlbum, user UserInfo) error {
	playlistId := user.PlaylistId
	client := CreateMyClientFromUserInfo(user).Client
	idSet := make(map[spotify.ID]struct{})
	pastTrackSet := make(map[spotify.ID]struct{})
	trackSet := make(map[string]struct{})
	var addTracks []spotify.ID
	var pastTracks []spotify.ID

	// get all the tracks in the playlist and put them in pastTrackSet
	playlistTrackPage, err := client.GetPlaylistTracks(spotify.ID(playlistId))
	if err != nil {
		return err
	}

	// keep records of the tracks already in the playlist and delete them all
	playlistTracks := playlistTrackPage.Tracks
	for _, track := range playlistTracks {
		pastTrackSet[track.Track.ID] = struct{}{}
		pastTracks = append(pastTracks, track.Track.ID)
	}
	if _, err := client.RemoveTracksFromPlaylist(spotify.ID(playlistId), pastTracks...); err != nil {
		return err
	}

	// retrieves track ids from newReleases. If album type is album, the first song in the album will
	// be in the playlist.
	for _, album := range newReleases {
		albumId := album.ID
		albumTracks, err := client.GetAlbumTracks(albumId)
		if err != nil {
			return err
		}
		fmt.Println(albumTracks.Tracks)
		track := albumTracks.Tracks[0]

		artist := track.Artists[0].Name
		trackName := track.Name
		identifier := artist + trackName

		trackId := track.ID
		
		if _, ok := idSet[trackId]; !ok { // prevent dupulicate tracks
			idSet[trackId] = struct{}{}
			if _, ok := pastTrackSet[trackId]; !ok { // prevent adding tracks which were added last week
				if _, ok := trackSet[identifier]; !ok { // prevent adding both explicit and non explicit track
					trackSet[identifier] = struct{}{}
					if ok := IfExclude(user, trackName); !ok { // exclude remix and track if required
						addTracks = append(addTracks, trackId)
					}
				}
			}
		}
		// time sleep is nessesary in order not to exceed spotify api limit
		time.Sleep(time.Millisecond * 500)
	}	
	fmt.Println(len(addTracks))
	// change the tracks in the playlist.
	change_num := (len(addTracks)-1) / 100 
	if change_num == 0 {
		if err := client.ReplacePlaylistTracks(spotify.ID(playlistId), addTracks...); err != nil {
			return err
		}
	}	else {
		if err := client.ReplacePlaylistTracks(spotify.ID(playlistId), addTracks[0:100]...); err != nil {
			return err
		}
	}
	
	for i := 0; i < change_num; i++ {
		var add []spotify.ID
		fmt.Println(i)
		if i == change_num-1 {
			add = addTracks[(i+1)*100:]
		} else {
			add = addTracks[(i+1)*100:(i+2)*100]
		}
		if _, err := client.AddTracksToPlaylist(spotify.ID(playlistId), add...); err != nil {
			return err
		}
	}

	return nil
}

func IfExclude(user UserInfo, trackName string) bool {
	res := false
	if user.IfRemixAdd == true && (strings.Contains(trackName, "Remix") || strings.Contains(trackName, "remix")) {
		res = true
	}
	if user.IfAcousticAdd == true && (strings.Contains(trackName, "Acoustic") || strings.Contains(trackName, "acoustic")) {
		res = true
	}
	return res 

}
