package engine

import (
	"sync/atomic"
	"time"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	infohash_v2 "github.com/anacrolix/torrent/types/infohash-v2"

	"github.com/ClearNetSky/Torrent/internal/safety"
)

// File priorities as exposed to the UI.
const (
	PrioritySkip   = 0
	PriorityNormal = 1
	PriorityHigh   = 2
)

// Transfer states as exposed to the UI.
const (
	StateMetadata    = "metadata"
	StateDownloading = "downloading"
	StateStalled     = "stalled"
	StateSeeding     = "seeding"
	StateCompleted   = "completed"
	StatePaused      = "paused"
	StateChecking    = "checking"
	StateError       = "error"
	StateOffline     = "offline"
)

// savedTorrent is the part of a transfer that survives restarts.
type savedTorrent struct {
	ID          string     `json:"id"`
	InfoHashV2  string     `json:"infoHashV2,omitempty"`
	Name        string     `json:"name"`
	SavePath    string     `json:"savePath"`
	Trackers    [][]string `json:"trackers,omitempty"`
	WebSeeds    []string   `json:"webSeeds,omitempty"`
	Priorities  []int      `json:"priorities,omitempty"`
	Paused      bool       `json:"paused,omitempty"`
	Finished    bool       `json:"finished,omitempty"`
	AddedAt     int64      `json:"addedAt"`
	CompletedAt int64      `json:"completedAt,omitempty"`
	Downloaded  int64      `json:"downloaded,omitempty"`
	Uploaded    int64      `json:"uploaded,omitempty"`
	DoneBytes   int64      `json:"doneBytes,omitempty"`
	WantedBytes int64      `json:"wantedBytes,omitempty"`
	TotalBytes  int64      `json:"totalBytes,omitempty"`
	Comment     string     `json:"comment,omitempty"`
	CreatedBy   string     `json:"createdBy,omitempty"`
	CreatedAt   int64      `json:"createdAt,omitempty"`
}

// item is one transfer. It exists whether or not it is loaded in a client:
// pausing drops it from the client entirely, so a paused torrent produces no
// network traffic at all.
type item struct {
	savedTorrent

	ih          metainfo.Hash
	ihV2        g.Option[infohash_v2.T]
	infoBytes   []byte
	rootName    string // raw info name, used for on-disk paths
	files       []fileMeta
	hasInfo     bool
	private     bool
	pieceLength int64
	numPieces   int

	t           *torrent.Torrent
	onPrivate   bool // loaded in the client that has DHT and PEX disabled
	sessRead    int64
	sessWritten int64
	lastSample  time.Time
	downRate    float64
	upRate      float64
	peers       int
	seeds       int
	knownPeers  int
	checking    bool
	err         string
	writeErr    atomic.Pointer[string]
	peerAddrs   []string
	dropped     []string // trackers skipped by the safety filter
}

func newItem(ih metainfo.Hash, v2 g.Option[infohash_v2.T]) *item {
	it := &item{ih: ih, ihV2: v2}
	it.ID = ih.HexString()
	if v2.Ok {
		it.InfoHashV2 = v2.Value.HexString()
	}
	return it
}

// setInfo records the info dictionary once it is known.
func (it *item) setInfo(infoBytes []byte, info *metainfo.Info) {
	it.infoBytes = infoBytes
	it.hasInfo = true
	it.files = buildFiles(info)
	it.private = info.Private != nil && *info.Private
	it.pieceLength = info.PieceLength
	it.numPieces = info.NumPieces()
	it.rootName = info.BestName()
	it.Name = safety.CleanText(info.BestName(), 512)
	it.TotalBytes = info.TotalLength()
	if len(it.Priorities) != len(it.files) {
		it.Priorities = make([]int, len(it.files))
		for i := range it.Priorities {
			it.Priorities[i] = PriorityNormal
		}
	}
	it.WantedBytes = it.wantedSize()
}

func (it *item) wantedSize() (n int64) {
	for i, f := range it.files {
		if !f.Padding && it.Priorities[i] != PrioritySkip {
			n += f.Length
		}
	}
	return n
}

func (it *item) allWanted() bool {
	for i, f := range it.files {
		if !f.Padding && it.Priorities[i] == PrioritySkip {
			return false
		}
	}
	return true
}

func (it *item) risky() bool {
	for i, f := range it.files {
		if f.Risky && !f.Padding && it.Priorities[i] != PrioritySkip {
			return true
		}
	}
	return false
}

func (it *item) wantsActive() bool {
	return !it.Paused && !it.Finished && it.err == ""
}

func (it *item) complete() bool {
	return it.hasInfo && it.DoneBytes >= it.WantedBytes
}

func (it *item) downloaded() int64 { return it.Downloaded + it.sessRead }

func (it *item) uploaded() int64 { return it.Uploaded + it.sessWritten }

func (it *item) ratio() float64 {
	d := it.downloaded()
	if d == 0 {
		// Data that was already on disk counts as the base for seeding.
		d = it.DoneBytes
	}
	if d == 0 {
		return 0
	}
	return float64(it.uploaded()) / float64(d)
}

func (it *item) state() string {
	switch {
	case it.err != "":
		return StateError
	case it.checking:
		return StateChecking
	case it.t == nil && it.Paused:
		return StatePaused
	case it.t == nil && it.Finished:
		return StateCompleted
	case it.t == nil:
		return StateOffline
	case !it.hasInfo:
		return StateMetadata
	case it.complete():
		return StateSeeding
	case it.peers == 0:
		return StateStalled
	default:
		return StateDownloading
	}
}

// TorrentView is the per-transfer row sent to the UI every second.
type TorrentView struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	State       string  `json:"state"`
	Progress    float64 `json:"progress"`
	Size        int64   `json:"size"`
	TotalSize   int64   `json:"totalSize"`
	Done        int64   `json:"done"`
	DownRate    int64   `json:"downRate"`
	UpRate      int64   `json:"upRate"`
	ETA         int64   `json:"eta"`
	Peers       int     `json:"peers"`
	Seeds       int     `json:"seeds"`
	KnownPeers  int     `json:"knownPeers"`
	Ratio       float64 `json:"ratio"`
	Downloaded  int64   `json:"downloaded"`
	Uploaded    int64   `json:"uploaded"`
	AddedAt     int64   `json:"addedAt"`
	CompletedAt int64   `json:"completedAt"`
	SavePath    string  `json:"savePath"`
	Error       string  `json:"error"`
	Private     bool    `json:"private"`
	HasMeta     bool    `json:"hasMeta"`
	Risky       bool    `json:"risky"`
}

func (it *item) view() TorrentView {
	state := it.state()
	v := TorrentView{
		ID:          it.ID,
		Name:        it.Name,
		State:       state,
		Size:        it.WantedBytes,
		TotalSize:   it.TotalBytes,
		Done:        it.DoneBytes,
		DownRate:    int64(it.downRate),
		UpRate:      int64(it.upRate),
		ETA:         -1,
		Peers:       it.peers,
		Seeds:       it.seeds,
		KnownPeers:  it.knownPeers,
		Ratio:       it.ratio(),
		Downloaded:  it.downloaded(),
		Uploaded:    it.uploaded(),
		AddedAt:     it.AddedAt,
		CompletedAt: it.CompletedAt,
		SavePath:    it.SavePath,
		Error:       it.err,
		Private:     it.private,
		HasMeta:     it.hasInfo,
		Risky:       it.risky(),
	}
	if it.WantedBytes > 0 {
		v.Progress = min(1, float64(it.DoneBytes)/float64(it.WantedBytes))
	} else if it.hasInfo {
		v.Progress = 1
	}
	if (state == StateDownloading || state == StateStalled) && it.downRate >= 1 {
		v.ETA = int64(float64(it.WantedBytes-it.DoneBytes) / it.downRate)
	}
	return v
}
