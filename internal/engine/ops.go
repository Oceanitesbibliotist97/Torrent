package engine

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	infohash_v2 "github.com/anacrolix/torrent/types/infohash-v2"

	"github.com/ClearNetSky/Torrent/internal/apperr"
	"github.com/ClearNetSky/Torrent/internal/config"
	"github.com/ClearNetSky/Torrent/internal/safety"
)

// AddOptions are the user's choices when adding a torrent.
type AddOptions struct {
	SavePath   string `json:"savePath"`
	Start      bool   `json:"start"`
	Priorities []int  `json:"priorities"` // per file, .torrent only; nil means all normal
}

// MagnetPreview describes a magnet link before it is added.
type MagnetPreview struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Trackers int    `json:"trackers"`
	Skipped  int    `json:"skipped"`
	Exists   bool   `json:"exists"`
}

func validateSavePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || !filepath.IsAbs(p) {
		return "", apperr.New(apperr.InvalidPath)
	}
	p = filepath.Clean(p)
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", apperr.Wrap(apperr.InvalidPath, err)
	}
	if st, err := os.Stat(p); err != nil || !st.IsDir() {
		return "", apperr.New(apperr.InvalidPath)
	}
	return p, nil
}

func clampPriority(p int) int {
	return max(PrioritySkip, min(p, PriorityHigh))
}

func magnetHashes(m metainfo.MagnetV2) (metainfo.Hash, g.Option[infohash_v2.T]) {
	if m.InfoHash.Ok {
		return m.InfoHash.Value, m.V2InfoHash
	}
	return *m.V2InfoHash.Value.ToShort(), m.V2InfoHash
}

// PreviewMagnet validates a magnet link and reports what adding it would do.
func (e *Engine) PreviewMagnet(uri string) (MagnetPreview, error) {
	m, err := safety.ParseMagnet(uri)
	if err != nil {
		return MagnetPreview{}, err
	}
	ih, _ := magnetHashes(m)
	e.mu.Lock()
	defer e.mu.Unlock()
	kept, dropped := safety.FilterTrackers([][]string{m.Trackers}, e.settings.NetworkMode != config.NetworkProxy)
	p := MagnetPreview{ID: ih.HexString(), Name: safety.CleanText(m.DisplayName, 512), Skipped: len(dropped)}
	for _, tier := range kept {
		p.Trackers += len(tier)
	}
	_, p.Exists = e.items[p.ID]
	return p, nil
}

// AddMagnet adds a transfer from a magnet link.
func (e *Engine) AddMagnet(uri string, opts AddOptions) (string, error) {
	m, err := safety.ParseMagnet(uri)
	if err != nil {
		return "", err
	}
	savePath, err := validateSavePath(opts.SavePath)
	if err != nil {
		return "", err
	}
	ih, v2 := magnetHashes(m)
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.items[ih.HexString()]; ok {
		return "", apperr.New(apperr.Duplicate)
	}
	it := newItem(ih, v2)
	it.Name = safety.CleanText(m.DisplayName, 512)
	if it.Name == "" {
		it.Name = it.ID
	}
	it.SavePath = savePath
	it.AddedAt = time.Now().UnixMilli()
	it.Trackers, _ = safety.FilterTrackers([][]string{m.Trackers}, true)
	it.WebSeeds = safety.FilterWebSeeds(m.Params["ws"])
	it.peerAddrs = safety.FilterPeerAddrs(m.Params["x.pe"])
	it.Paused = !opts.Start
	e.items[it.ID] = it
	e.addedLocked(it)
	return it.ID, nil
}

// PreviewTorrentFile reads a .torrent file for the add dialog.
func (e *Engine) PreviewTorrentFile(path string) (Preview, error) {
	mi, info, err := loadTorrentFile(path)
	if err != nil {
		return Preview{}, err
	}
	ih, _ := torrentHashes(mi.InfoBytes, info)
	p := Preview{
		ID:        ih.HexString(),
		Path:      path,
		Name:      safety.CleanText(info.BestName(), 512),
		TotalSize: info.TotalLength(),
		Private:   info.Private != nil && *info.Private,
		Comment:   safety.CleanMultiline(mi.Comment, 2000),
		CreatedBy: safety.CleanText(mi.CreatedBy, 200),
		CreatedAt: mi.CreationDate,
		Files:     []PreviewFile{},
	}
	for _, tier := range mi.UpvertedAnnounceList() {
		p.Trackers += len(tier)
	}
	files := buildFiles(info)
	p.FileCount = len(files)
	for i, f := range files {
		if f.Padding {
			continue
		}
		p.Files = append(p.Files, PreviewFile{Index: i, Path: f.Path, Size: f.Length, Risky: f.Risky})
		if f.Risky {
			p.RiskyFiles++
		}
	}
	e.mu.Lock()
	_, p.Exists = e.items[p.ID]
	e.mu.Unlock()
	return p, nil
}

// AddTorrentFile adds a transfer from a .torrent file. The file is parsed
// again rather than trusting anything from the preview.
func (e *Engine) AddTorrentFile(path string, opts AddOptions) (string, error) {
	mi, info, err := loadTorrentFile(path)
	if err != nil {
		return "", err
	}
	savePath, err := validateSavePath(opts.SavePath)
	if err != nil {
		return "", err
	}
	ih, v2 := torrentHashes(mi.InfoBytes, info)
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.items[ih.HexString()]; ok {
		return "", apperr.New(apperr.Duplicate)
	}
	it := newItem(ih, v2)
	it.SavePath = savePath
	it.AddedAt = time.Now().UnixMilli()
	it.Trackers, _ = safety.FilterTrackers(mi.UpvertedAnnounceList(), true)
	it.WebSeeds = safety.FilterWebSeeds(mi.UrlList)
	it.Comment = safety.CleanMultiline(mi.Comment, 2000)
	it.CreatedBy = safety.CleanText(mi.CreatedBy, 200)
	it.CreatedAt = mi.CreationDate
	it.setInfo(mi.InfoBytes, info)
	if len(opts.Priorities) == len(it.files) {
		for i, p := range opts.Priorities {
			it.Priorities[i] = clampPriority(p)
		}
		it.WantedBytes = it.wantedSize()
	}
	it.Paused = !opts.Start
	e.items[it.ID] = it
	e.addedLocked(it)
	return it.ID, nil
}

func (e *Engine) addedLocked(it *item) {
	if it.wantsActive() && e.client != nil {
		if err := e.activateLocked(it); err != nil {
			it.err = err.Error()
		}
	}
	e.writeMetainfoLocked(it)
	e.saveSessionLocked()
}

// Pause drops transfers from the client, so they generate no traffic at all.
func (e *Engine) Pause(ids []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, id := range ids {
		if it, ok := e.items[id]; ok {
			it.Paused = true
			e.deactivateLocked(it)
		}
	}
	e.saveSessionLocked()
}

// Resume restarts paused, finished or failed transfers. While offline they
// start as soon as the network is available again.
func (e *Engine) Resume(ids []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var firstErr error
	for _, id := range ids {
		it, ok := e.items[id]
		if !ok {
			continue
		}
		it.Paused, it.Finished, it.err = false, false, ""
		if e.client == nil {
			continue
		}
		if err := e.activateLocked(it); err != nil {
			it.err = err.Error()
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	e.saveSessionLocked()
	return firstErr
}

// Remove deletes transfers and, optionally, their downloaded files.
func (e *Engine) Remove(ids []string, deleteFiles bool) error {
	type job struct {
		savePath, root string
		files          []fileMeta
	}
	var jobs []job
	e.mu.Lock()
	for _, id := range ids {
		it, ok := e.items[id]
		if !ok {
			continue
		}
		e.deactivateLocked(it)
		delete(e.items, id)
		e.forgetLocked(it)
		if deleteFiles && it.hasInfo {
			jobs = append(jobs, job{it.SavePath, it.rootName, it.files})
		}
	}
	e.saveSessionLocked()
	e.mu.Unlock()

	var firstErr error
	for _, j := range jobs {
		if err := removeContent(j.savePath, j.root, j.files); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SetFilePriority changes the priority of some files of a transfer.
func (e *Engine) SetFilePriority(id string, indices []int, priority int) error {
	priority = clampPriority(priority)
	e.mu.Lock()
	defer e.mu.Unlock()
	it, ok := e.items[id]
	if !ok {
		return apperr.New(apperr.NotFound)
	}
	if !it.hasInfo {
		return apperr.New(apperr.InvalidInput)
	}
	for _, i := range indices {
		if i >= 0 && i < len(it.Priorities) {
			it.Priorities[i] = priority
		}
	}
	it.WantedBytes = it.wantedSize()
	if it.t != nil {
		e.applyPrioritiesLocked(it)
		it.DoneBytes = bytesDone(it)
	} else if it.Finished && priority != PrioritySkip {
		// Newly selected files of a finished transfer need downloading.
		it.Finished = false
		if !it.Paused && e.client != nil {
			if err := e.activateLocked(it); err != nil {
				it.err = err.Error()
			}
		}
	}
	e.dirty = true
	return nil
}

// Recheck verifies downloaded data against the piece hashes.
func (e *Engine) Recheck(id string) error {
	e.mu.Lock()
	it, ok := e.items[id]
	if !ok {
		e.mu.Unlock()
		return apperr.New(apperr.NotFound)
	}
	t := it.t
	if t == nil || !it.hasInfo {
		e.mu.Unlock()
		return apperr.New(apperr.InvalidInput)
	}
	if it.checking {
		e.mu.Unlock()
		return nil
	}
	it.checking = true
	e.mu.Unlock()
	go func() {
		err := t.VerifyData()
		e.mu.Lock()
		defer e.mu.Unlock()
		if it.t == t {
			it.checking = false
			if err != nil {
				e.noticeLocked("error", "notice.recheckFailed", it.Name)
			}
		}
	}()
	return nil
}

// OpenFolder shows a transfer's files in the system file manager.
func (e *Engine) OpenFolder(id string) error {
	e.mu.Lock()
	it, ok := e.items[id]
	if !ok {
		e.mu.Unlock()
		return apperr.New(apperr.NotFound)
	}
	target, isDir := it.SavePath, true
	if it.hasInfo {
		root := contentRoot(it.SavePath, it.rootName)
		if st, err := os.Stat(root); err == nil && safety.WithinDir(it.SavePath, root) {
			target, isDir = root, st.IsDir()
		}
	}
	e.mu.Unlock()
	if err := revealInFileManager(target, isDir); err != nil {
		return apperr.Wrap(apperr.IO, err)
	}
	return nil
}

// MagnetLink builds a shareable magnet link for a transfer.
func (e *Engine) MagnetLink(id string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	it, ok := e.items[id]
	if !ok {
		return "", apperr.New(apperr.NotFound)
	}
	var trackers []string
	for _, tier := range it.Trackers {
		trackers = append(trackers, tier...)
	}
	if len(trackers) > 10 {
		trackers = trackers[:10]
	}
	m := metainfo.MagnetV2{DisplayName: it.Name, Trackers: trackers, V2InfoHash: it.ihV2}
	// A v2-only torrent's ID is a truncated v2 hash, not a real v1 hash.
	if !it.ihV2.Ok || *it.ihV2.Value.ToShort() != it.ih {
		m.InfoHash.Set(it.ih)
	}
	return m.String(), nil
}

// FileView is one file in the details panel. Done is -1 when unknown.
type FileView struct {
	Index    int    `json:"index"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Done     int64  `json:"done"`
	Priority int    `json:"priority"`
	Risky    bool   `json:"risky"`
}

// PeerView is one connected peer.
type PeerView struct {
	Address  string  `json:"address"`
	Client   string  `json:"client"`
	Network  string  `json:"network"`
	Source   string  `json:"source"`
	Progress float64 `json:"progress"`
	DownRate int64   `json:"downRate"`
	UpRate   int64   `json:"upRate"`
}

// TrackerView is one tracker URL. Skipped trackers were filtered for safety,
// for example UDP trackers while a proxy is in use.
type TrackerView struct {
	URL     string `json:"url"`
	Tier    int    `json:"tier"`
	Skipped bool   `json:"skipped"`
}

// Details is the data for the details panel. Only the requested section is
// filled, to keep the once-a-second refresh cheap for huge torrents.
type Details struct {
	ID          string        `json:"id"`
	InfoHashV2  string        `json:"infoHashV2"`
	Name        string        `json:"name"`
	SavePath    string        `json:"savePath"`
	Comment     string        `json:"comment"`
	CreatedBy   string        `json:"createdBy"`
	CreatedAt   int64         `json:"createdAt"`
	PieceLength int64         `json:"pieceLength"`
	Pieces      int           `json:"pieces"`
	Private     bool          `json:"private"`
	Files       []FileView    `json:"files"`
	Peers       []PeerView    `json:"peers"`
	Trackers    []TrackerView `json:"trackers"`
}

const maxPeerViews = 200

// Details returns one section ("overview", "files", "peers" or "trackers").
func (e *Engine) Details(id, section string) (Details, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	it, ok := e.items[id]
	if !ok {
		return Details{}, apperr.New(apperr.NotFound)
	}
	d := Details{
		ID:          it.ID,
		InfoHashV2:  it.InfoHashV2,
		Name:        it.Name,
		SavePath:    it.SavePath,
		Comment:     it.Comment,
		CreatedBy:   it.CreatedBy,
		CreatedAt:   it.CreatedAt,
		PieceLength: it.pieceLength,
		Pieces:      it.numPieces,
		Private:     it.private,
		Files:       []FileView{},
		Peers:       []PeerView{},
		Trackers:    []TrackerView{},
	}
	switch section {
	case "files":
		var live []*torrent.File
		if it.t != nil && it.hasInfo {
			live = it.t.Files()
		}
		for i, f := range it.files {
			if f.Padding {
				continue
			}
			fv := FileView{Index: i, Path: f.Path, Size: f.Length, Done: -1, Priority: it.Priorities[i], Risky: f.Risky}
			switch {
			case i < len(live):
				fv.Done = live[i].BytesCompleted()
			case it.complete() && it.Priorities[i] != PrioritySkip:
				fv.Done = f.Length
			}
			d.Files = append(d.Files, fv)
		}
	case "peers":
		if it.t == nil {
			break
		}
		for _, pc := range it.t.PeerConns() {
			if len(d.Peers) == maxPeerViews {
				break
			}
			st := pc.Stats()
			name, _ := pc.PeerClientName.Load().(string)
			pv := PeerView{
				Address:  pc.RemoteAddr.String(),
				Client:   safety.CleanText(name, 64),
				Network:  pc.Network,
				Source:   string(pc.Discovery),
				DownRate: int64(st.DownloadRate),
				UpRate:   int64(st.LastWriteUploadRate),
			}
			if it.numPieces > 0 {
				pv.Progress = min(1, float64(st.RemotePieceCount)/float64(it.numPieces))
			}
			d.Peers = append(d.Peers, pv)
		}
	case "trackers":
		skipped := make(map[string]bool, len(it.dropped))
		for _, u := range it.dropped {
			skipped[u] = true
		}
		for tier, urls := range it.Trackers {
			for _, u := range urls {
				d.Trackers = append(d.Trackers, TrackerView{URL: u, Tier: tier, Skipped: skipped[u]})
			}
		}
	}
	return d, nil
}

// activateLocked loads a transfer into the public or the private client.
func (e *Engine) activateLocked(it *item) error {
	if it.t != nil {
		return nil
	}
	if e.client == nil {
		return apperr.New(apperr.EngineOffline)
	}
	cl := e.client
	if it.private {
		if e.privClient == nil {
			pc, err := e.newClientLocked(e.plan, true)
			if err != nil {
				return err
			}
			e.privClient = pc
		}
		cl = e.privClient
	}
	t, _ := cl.AddTorrentOpt(torrent.AddTorrentOpts{
		InfoHash:   it.ih,
		InfoHashV2: it.ihV2,
		Storage:    e.storageLocked(it.SavePath),
		InfoBytes:  it.infoBytes,
	})
	tiers, dropped := safety.FilterTrackers(it.Trackers, e.plan.mode != config.NetworkProxy)
	it.dropped = dropped
	spec := &torrent.TorrentSpec{Trackers: tiers, PeerAddrs: it.peerAddrs}
	if e.settings.EnableWebSeeds {
		spec.Webseeds = it.WebSeeds
	}
	if !it.hasInfo {
		spec.DisplayName = it.Name
	}
	if err := t.MergeSpec(spec); err != nil {
		t.Drop()
		return apperr.Wrap(apperr.InvalidTorrent, err)
	}
	it.peerAddrs = nil
	it.t = t
	it.onPrivate = cl == e.privClient
	it.lastSample = time.Time{}
	it.writeErr.Store(nil)
	t.SetOnWriteChunkError(func(err error) {
		msg := err.Error()
		it.writeErr.Store(&msg)
	})
	if it.hasInfo {
		e.applyPrioritiesLocked(it)
	}
	return nil
}

// deactivateLocked drops a transfer from its client, folding this session's
// transfer counters into the persistent totals.
func (e *Engine) deactivateLocked(it *item) {
	t := it.t
	if t == nil {
		return
	}
	if it.hasInfo {
		it.DoneBytes = bytesDone(it)
	}
	stats := t.Stats()
	it.Downloaded += stats.BytesReadUsefulData.Int64()
	it.Uploaded += stats.BytesWrittenData.Int64()
	it.sessRead, it.sessWritten = 0, 0
	it.t = nil
	it.onPrivate = false
	it.downRate, it.upRate = 0, 0
	it.peers, it.seeds, it.knownPeers = 0, 0, 0
	it.checking = false
	t.Drop()
	e.dirty = true
}

func (e *Engine) applyPrioritiesLocked(it *item) {
	if it.t == nil || !it.hasInfo {
		return
	}
	for i, f := range it.t.Files() {
		if i >= len(it.Priorities) {
			break
		}
		p := torrent.PiecePriorityNormal
		switch it.Priorities[i] {
		case PrioritySkip:
			p = torrent.PiecePriorityNone
		case PriorityHigh:
			p = torrent.PiecePriorityHigh
		}
		f.SetPriority(p)
	}
}

func (it *item) hasPadding() bool {
	for _, f := range it.files {
		if f.Padding {
			return true
		}
	}
	return false
}

// wantedPaths lists the on-disk paths of the selected files.
func (it *item) wantedPaths() []string {
	var out []string
	for i, f := range it.files {
		if f.Padding || it.Priorities[i] == PrioritySkip {
			continue
		}
		if p := filePath(it.SavePath, it.rootName, f); safety.WithinDir(it.SavePath, p) {
			out = append(out, p)
		}
	}
	return out
}
