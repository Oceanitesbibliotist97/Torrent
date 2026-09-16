// Command winres renders the application icon and writes the Windows resource
// object (icon, manifest, version info) that go build links into Torrent.exe.
//
//	go run ./tools/winres -version 1.0.0
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
	"golang.org/x/image/vector"

	"github.com/ClearNetSky/Torrent/internal/appinfo"
)

func main() {
	ver := flag.String("version", appinfo.Version, "product version (x.y.z)")
	flag.Parse()

	var images []image.Image
	for _, size := range []int{256, 128, 64, 48, 40, 32, 24, 20, 16} {
		images = append(images, renderLogo(size))
	}
	icon, err := winres.NewIconFromImages(images)
	if err != nil {
		log.Fatal(err)
	}

	must(os.MkdirAll(filepath.Join("build", "windows"), 0o755))
	writePNG(filepath.Join("build", "appicon.png"), renderLogo(512))
	icoFile, err := os.Create(filepath.Join("build", "windows", "icon.ico"))
	must(err)
	must(icon.SaveICO(icoFile))
	must(icoFile.Close())

	rs := winres.ResourceSet{}
	must(rs.SetIcon(winres.Name("APP"), icon))
	rs.SetManifest(winres.AppManifest{
		Identity:            winres.AssemblyIdentity{Name: appinfo.ID, Version: [4]uint16{}},
		Description:         appinfo.Name,
		Compatibility:       winres.Win10AndAbove,
		ExecutionLevel:      winres.AsInvoker,
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
		LongPathAware:       true,
	})

	fileVersion := numericVersion(*ver)
	vi := version.Info{}
	vi.SetFileVersion(fileVersion)
	vi.SetProductVersion(fileVersion)
	for key, value := range map[string]string{
		version.ProductName:      appinfo.Name,
		version.FileDescription:  appinfo.Name + " — private portable BitTorrent client",
		version.ProductVersion:   *ver,
		version.FileVersion:      *ver,
		version.OriginalFilename: "Torrent.exe",
		version.InternalName:     "Torrent",
		version.LegalCopyright:   "Free and open source software",
	} {
		must(vi.Set(version.LangDefault, key, value))
	}
	rs.SetVersionInfo(vi)

	obj, err := os.Create("rsrc_windows_amd64.syso")
	must(err)
	must(rs.WriteObject(obj, winres.ArchAMD64))
	must(obj.Close())
	fmt.Println("wrote rsrc_windows_amd64.syso, build/windows/icon.ico, build/appicon.png")
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func numericVersion(v string) string {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	for len(parts) < 4 {
		parts = append(parts, "0")
	}
	return strings.Join(parts[:4], ".")
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	must(err)
	must(png.Encode(f, img))
	must(f.Close())
}

/* ----------------------------------------------------------- Rendering */

// gradient is the diagonal brand gradient from #7d88ff to #5836d6.
type gradient struct{ size int }

func (g gradient) ColorModel() color.Model { return color.RGBAModel }
func (g gradient) Bounds() image.Rectangle { return image.Rect(0, 0, g.size, g.size) }
func (g gradient) At(x, y int) color.Color {
	t := float64(x+y) / float64(2*g.size)
	lerp := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t)) }
	return color.RGBA{lerp(0x7d, 0x58), lerp(0x88, 0x36), lerp(0xff, 0xd6), 0xff}
}

const k = 0.5523 // cubic Bézier approximation of a quarter circle

func roundedRect(r *vector.Rasterizer, x0, y0, x1, y1, rad float32) {
	r.MoveTo(x0+rad, y0)
	r.LineTo(x1-rad, y0)
	r.CubeTo(x1-rad+rad*k, y0, x1, y0+rad-rad*k, x1, y0+rad)
	r.LineTo(x1, y1-rad)
	r.CubeTo(x1, y1-rad+rad*k, x1-rad+rad*k, y1, x1-rad, y1)
	r.LineTo(x0+rad, y1)
	r.CubeTo(x0+rad-rad*k, y1, x0, y1-rad+rad*k, x0, y1-rad)
	r.LineTo(x0, y0+rad)
	r.CubeTo(x0, y0+rad-rad*k, x0+rad-rad*k, y0, x0+rad, y0)
	r.ClosePath()
}

// shield draws the logo shield scaled around its centre by (sx, sy).
func shield(r *vector.Rasterizer, s, sx, sy float32) {
	const cx, cy = 32, 31.5
	p := func(x, y float32) (float32, float32) {
		return (cx + (x-cx)*sx) * s, (cy + (y-cy)*sy) * s
	}
	r.MoveTo(p(32, 12))
	r.LineTo(p(17, 18))
	r.LineTo(p(17, 29))
	x1, y1 := p(17, 39)
	x2, y2 := p(23.4, 47.6)
	x3, y3 := p(32, 51)
	r.CubeTo(x1, y1, x2, y2, x3, y3)
	x1, y1 = p(40.6, 47.6)
	x2, y2 = p(47, 39)
	x3, y3 = p(47, 29)
	r.CubeTo(x1, y1, x2, y2, x3, y3)
	r.LineTo(p(47, 18))
	r.ClosePath()
}

func circle(r *vector.Rasterizer, cx, cy, rad float32) {
	const n = 28
	r.MoveTo(cx+rad, cy)
	for i := 1; i < n; i++ {
		a := 2 * math.Pi * float64(i) / n
		r.LineTo(cx+rad*float32(math.Cos(a)), cy+rad*float32(math.Sin(a)))
	}
	r.ClosePath()
}

// line draws a thick segment with round caps.
func line(r *vector.Rasterizer, x0, y0, x1, y1, w float32) {
	dx, dy := x1-x0, y1-y0
	l := float32(math.Hypot(float64(dx), float64(dy)))
	nx, ny := -dy/l*w/2, dx/l*w/2
	r.MoveTo(x0+nx, y0+ny)
	r.LineTo(x0-nx, y0-ny)
	r.LineTo(x1-nx, y1-ny)
	r.LineTo(x1+nx, y1+ny)
	r.ClosePath()
	circle(r, x0, y0, w/2)
	circle(r, x1, y1, w/2)
}

func renderLogo(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float32(size) / 64
	white := image.NewUniform(color.RGBA{0xff, 0xff, 0xff, 0xff})
	tint := image.NewUniform(color.NRGBA{0xff, 0xff, 0xff, 0x24})
	grad := gradient{size}
	var r vector.Rasterizer

	draw := func(src image.Image, build func()) {
		r.Reset(size, size)
		build()
		r.Draw(img, img.Bounds(), src, image.Point{})
	}

	// Small icons get a bolder mark so it stays legible.
	stroke := float32(3.4)
	if size <= 32 {
		stroke = 5
	}
	draw(grad, func() { roundedRect(&r, 2*s, 2*s, 62*s, 62*s, 16*s) })
	draw(white, func() { shield(&r, s, 1+stroke/2/15, 1+stroke/2/19.5) })
	draw(grad, func() { shield(&r, s, 1-stroke/2/15, 1-stroke/2/19.5) })
	draw(tint, func() { shield(&r, s, 1-stroke/2/15, 1-stroke/2/19.5) })
	arrow := stroke + 0.2
	draw(white, func() {
		line(&r, 32*s, 23.5*s, 32*s, 39*s, arrow*s)
		line(&r, 25.5*s, 33*s, 32*s, 39.5*s, arrow*s)
		line(&r, 32*s, 39.5*s, 38.5*s, 33*s, arrow*s)
	})
	return img
}
