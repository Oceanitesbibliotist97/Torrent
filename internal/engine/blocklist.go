package engine

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"

	"github.com/anacrolix/torrent/iplist"

	"github.com/ClearNetSky/Torrent/internal/apperr"
)

const maxBlocklistSize = 256 << 20

// loadBlocklist parses a P2P plaintext blocklist ("name:1.2.3.4-1.2.3.9"),
// optionally gzip-compressed. The file is read locally; nothing is downloaded.
func loadBlocklist(path string) (*iplist.IPList, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, apperr.Wrap(apperr.Blocklist, err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, apperr.Wrap(apperr.Blocklist, err)
	}
	if !st.Mode().IsRegular() || st.Size() > maxBlocklistSize {
		return nil, apperr.New(apperr.Blocklist)
	}
	br := bufio.NewReader(f)
	var r io.Reader = br
	if magic, err := br.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, apperr.Wrap(apperr.Blocklist, err)
		}
		defer gz.Close()
		r = io.LimitReader(gz, 4*maxBlocklistSize)
	}
	list, err := iplist.NewFromReader(r)
	if err != nil {
		return nil, apperr.Wrap(apperr.Blocklist, err)
	}
	if list.NumRanges() == 0 {
		return nil, apperr.Detail(apperr.Blocklist, "no IP ranges found")
	}
	return list, nil
}
