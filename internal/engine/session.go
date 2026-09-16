package engine

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent/metainfo"
	infohash_v2 "github.com/anacrolix/torrent/types/infohash-v2"

	"github.com/ClearNetSky/Torrent/internal/fsutil"
	"github.com/ClearNetSky/Torrent/internal/safety"
)

const (
	sessionFile    = "session.json"
	resumeDBFile   = "resume.db"
	torrentsDir    = "torrents"
	sessionVersion = 1
)

type sessionData struct {
	Version  int            `json:"version"`
	Torrents []savedTorrent `json:"torrents"`
}

func (e *Engine) saveSessionLocked() {
	e.dirty = false
	e.lastSave = time.Now()
	if !e.settings.RememberTorrents {
		return
	}
	data := sessionData{Version: sessionVersion, Torrents: []savedTorrent{}}
	for _, it := range e.sortedItemsLocked() {
		st := it.savedTorrent
		st.Downloaded, st.Uploaded = it.downloaded(), it.uploaded()
		data.Torrents = append(data.Torrents, st)
	}
	b, err := json.Marshal(data)
	if err != nil {
		return
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(e.dataDir, sessionFile), b, 0o600); err != nil {
		e.noticeLocked("error", "notice.saveFailed", "")
	}
}

func (e *Engine) loadSessionLocked() error {
	b, err := os.ReadFile(filepath.Join(e.dataDir, sessionFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var data sessionData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	for _, st := range data.Torrents {
		if !safety.IsTorrentID(st.ID) || len(st.ID) != 40 || !filepath.IsAbs(st.SavePath) {
			continue
		}
		ih := metainfo.NewHashFromHex(st.ID)
		var v2 g.Option[infohash_v2.T]
		if st.InfoHashV2 != "" {
			var h infohash_v2.T
			if err := h.FromHexString(st.InfoHashV2); err == nil {
				v2.Set(h)
			}
		}
		it := newItem(ih, v2)
		it.savedTorrent = st
		it.Name = safety.CleanText(st.Name, 512)
		if mi, err := metainfo.LoadFromFile(e.metainfoPath(st.ID)); err == nil {
			if info, err := parseInfo(mi.InfoBytes); err == nil {
				if got, _ := torrentHashes(mi.InfoBytes, info); got == ih {
					it.setInfo(mi.InfoBytes, info)
				}
			}
		}
		e.items[it.ID] = it
	}
	return nil
}

func (e *Engine) metainfoPath(id string) string {
	return filepath.Join(e.dataDir, torrentsDir, id+".torrent")
}

// writeMetainfoLocked keeps a copy of the metainfo so magnet links do not
// need to fetch metadata again after a restart.
func (e *Engine) writeMetainfoLocked(it *item) {
	if !e.settings.RememberTorrents || it.infoBytes == nil {
		return
	}
	mi := metainfo.MetaInfo{
		InfoBytes:    it.infoBytes,
		AnnounceList: it.Trackers,
		UrlList:      it.WebSeeds,
		Comment:      it.Comment,
		CreatedBy:    it.CreatedBy,
		CreationDate: it.CreatedAt,
	}
	var buf bytes.Buffer
	if err := mi.Write(&buf); err != nil {
		return
	}
	_ = fsutil.WriteFileAtomic(e.metainfoPath(it.ID), buf.Bytes(), 0o600)
}

// forgetLocked erases every local record of a removed transfer.
func (e *Engine) forgetLocked(it *item) {
	os.Remove(e.metainfoPath(it.ID))
	_ = e.completion.Forget(it.ih)
}

// wipeHistoryFiles deletes everything that records which torrents were used.
// The resume database can only be deleted once it is closed.
func (e *Engine) wipeHistoryFiles(includeResumeDB bool) {
	os.Remove(filepath.Join(e.dataDir, sessionFile))
	os.RemoveAll(filepath.Join(e.dataDir, torrentsDir))
	if includeResumeDB {
		os.Remove(filepath.Join(e.dataDir, resumeDBFile))
	}
}
