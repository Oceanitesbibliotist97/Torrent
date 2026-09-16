package engine

import (
	"cmp"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent/metainfo"
	infohash_v2 "github.com/anacrolix/torrent/types/infohash-v2"

	"github.com/ClearNetSky/Torrent/internal/apperr"
	"github.com/ClearNetSky/Torrent/internal/safety"
)

// maxFiles bounds the number of files accepted in one torrent.
const maxFiles = 200_000

// Preview describes a .torrent file before it is added.
type Preview struct {
	ID         string        `json:"id"`
	Path       string        `json:"path"`
	Name       string        `json:"name"`
	TotalSize  int64         `json:"totalSize"`
	FileCount  int           `json:"fileCount"` // including hidden padding files; priorities use this length
	Files      []PreviewFile `json:"files"`
	Private    bool          `json:"private"`
	Comment    string        `json:"comment"`
	CreatedBy  string        `json:"createdBy"`
	CreatedAt  int64         `json:"createdAt"`
	Trackers   int           `json:"trackers"`
	RiskyFiles int           `json:"riskyFiles"`
	Exists     bool          `json:"exists"`
}

// PreviewFile is one file listed in a Preview.
type PreviewFile struct {
	Index int    `json:"index"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Risky bool   `json:"risky"`
}

// fileMeta is the static description of one file in a torrent.
type fileMeta struct {
	Path    string   // display path, safe to show
	parts   []string // raw path components below the torrent root
	Length  int64
	Padding bool // BEP 47 padding file, never shown or written
	Risky   bool
}

func buildFiles(info *metainfo.Info) []fileMeta {
	fis := info.UpvertedFiles()
	out := make([]fileMeta, len(fis))
	for i := range fis {
		fi := &fis[i]
		parts := fi.BestPath()
		base := info.BestName()
		if len(parts) > 0 {
			base = parts[len(parts)-1]
		}
		out[i] = fileMeta{
			Path:    safety.CleanText(fi.DisplayPath(info), 4096),
			parts:   parts,
			Length:  fi.Length,
			Padding: strings.Contains(fi.Attr, "p"),
			Risky:   safety.RiskyFileName(base),
		}
	}
	return out
}

// loadTorrentFile reads and validates a .torrent file with strict size limits.
func loadTorrentFile(path string) (*metainfo.MetaInfo, *metainfo.Info, error) {
	if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".torrent") {
		return nil, nil, apperr.New(apperr.InvalidTorrent)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, apperr.Wrap(apperr.IO, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, nil, apperr.Wrap(apperr.IO, err)
	}
	if !st.Mode().IsRegular() {
		return nil, nil, apperr.New(apperr.InvalidTorrent)
	}
	if st.Size() > safety.MaxTorrentFileSize {
		return nil, nil, apperr.New(apperr.TooLarge)
	}
	mi, err := metainfo.Load(io.LimitReader(f, safety.MaxTorrentFileSize))
	if err != nil {
		return nil, nil, apperr.New(apperr.InvalidTorrent)
	}
	info, err := parseInfo(mi.InfoBytes)
	if err != nil {
		return nil, nil, err
	}
	return mi, info, nil
}

// parseInfo decodes and sanity-checks an info dictionary.
func parseInfo(infoBytes []byte) (*metainfo.Info, error) {
	if len(infoBytes) == 0 {
		return nil, apperr.New(apperr.InvalidTorrent)
	}
	mi := metainfo.MetaInfo{InfoBytes: infoBytes}
	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, apperr.New(apperr.InvalidTorrent)
	}
	total := info.TotalLength()
	if info.PieceLength <= 0 || total <= 0 {
		return nil, apperr.New(apperr.InvalidTorrent)
	}
	if len(info.UpvertedFiles()) > maxFiles {
		return nil, apperr.New(apperr.TooLarge)
	}
	if info.HasV1() {
		if len(info.Pieces)%20 != 0 {
			return nil, apperr.New(apperr.InvalidTorrent)
		}
		covered := int64(len(info.Pieces)/20) * info.PieceLength
		if covered < total || covered-total >= info.PieceLength {
			return nil, apperr.New(apperr.InvalidTorrent)
		}
	}
	return &info, nil
}

// torrentHashes derives the short (v1 or truncated v2) info hash used as the
// transfer ID, plus the full v2 hash when present.
func torrentHashes(infoBytes []byte, info *metainfo.Info) (metainfo.Hash, g.Option[infohash_v2.T]) {
	var v1 metainfo.Hash
	var v2 g.Option[infohash_v2.T]
	if info.HasV1() {
		v1 = metainfo.HashBytes(infoBytes)
	}
	if info.HasV2() {
		v2.Set(infohash_v2.HashBytes(infoBytes))
	}
	if v1 == (metainfo.Hash{}) && v2.Ok {
		v1 = *v2.Value.ToShort()
	}
	return v1, v2
}

// contentRoot is where a torrent's files live: savePath/name for torrents
// with a name, or savePath itself for nameless ones.
func contentRoot(savePath, name string) string {
	if name == "" || name == metainfo.NoName {
		return savePath
	}
	return filepath.Join(savePath, name)
}

func filePath(savePath, name string, f fileMeta) string {
	return filepath.Join(append([]string{contentRoot(savePath, name)}, f.parts...)...)
}

// removeContent deletes a torrent's files and prunes directories that become
// empty. Paths are checked lexically and after resolving links, so nothing
// outside savePath can be touched even through junctions.
func removeContent(savePath, name string, files []fileMeta) error {
	realBase, err := filepath.EvalSymlinks(savePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return apperr.Wrap(apperr.IO, err)
	}
	dirs := make(map[string]struct{})
	var firstErr error
	for _, f := range files {
		if f.Padding {
			continue
		}
		p := filePath(savePath, name, f)
		if !safety.WithinDir(savePath, p) {
			continue
		}
		realDir, err := filepath.EvalSymlinks(filepath.Dir(p))
		if err != nil || (realDir != realBase && !safety.WithinDir(realBase, realDir)) {
			continue
		}
		if err := removeWithRetry(p); err != nil && !errors.Is(err, fs.ErrNotExist) && firstErr == nil {
			firstErr = apperr.Wrap(apperr.IO, err)
		}
		for d := filepath.Dir(p); safety.WithinDir(savePath, d); d = filepath.Dir(d) {
			dirs[d] = struct{}{}
		}
	}
	sorted := make([]string, 0, len(dirs))
	for d := range dirs {
		sorted = append(sorted, d)
	}
	// Deepest first; removing a non-empty directory fails harmlessly.
	slices.SortFunc(sorted, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	for _, d := range sorted {
		os.Remove(d)
	}
	return firstErr
}

// removeWithRetry deletes a file that may still be open. The torrent library
// occasionally leaves an *os.File for the garbage collector to finalize, and on
// Windows that handle blocks deletion, so a collection is forced before
// retrying. Antivirus scanning a fresh download can also hold files briefly.
func removeWithRetry(path string) error {
	delay := 100 * time.Millisecond
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		err = os.Remove(path)
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			return err
		}
		runtime.GC()
		time.Sleep(delay)
		delay = min(delay*2, 1600*time.Millisecond)
	}
	return err
}
