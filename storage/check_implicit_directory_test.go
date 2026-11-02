package storage

import (
	"archive/tar"
	"strings"
	"testing"
	"time"
)

func implicitCheckHeader(n string, kind byte, mt int64) *tar.Header {
	return &tar.Header{Name: n, Typeflag: kind, Mode: 0o755, Uid: 0, Gid: 0, ModTime: time.Unix(mt, 0)}
}

func TestImplicitDirectoryImportTimes(t *testing.T) {
	for _, tc := range []struct {
		name         string
		base, change []*tar.Header
		which        string
	}{{"existing parent", []*tar.Header{implicitCheckHeader("etc/", tar.TypeDir, 10)}, []*tar.Header{implicitCheckHeader("etc/tool", tar.TypeReg, 20)}, "etc"}, {"new parent", nil, []*tar.Header{implicitCheckHeader("etc/tool", tar.TypeReg, 0)}, "etc"}, {"nested existing parent", []*tar.Header{implicitCheckHeader("etc/", tar.TypeDir, 10), implicitCheckHeader("etc/sub/", tar.TypeDir, 11)}, []*tar.Header{implicitCheckHeader("etc/sub/tool", tar.TypeReg, 20)}, "etc"}, {"nested new parent", []*tar.Header{implicitCheckHeader("etc/", tar.TypeDir, 10)}, []*tar.Header{implicitCheckHeader("etc/new/tool", tar.TypeReg, 20)}, "etc"}, {"whiteout changes parent", []*tar.Header{implicitCheckHeader("etc/", tar.TypeDir, 10), implicitCheckHeader("etc/old", tar.TypeReg, 10)}, []*tar.Header{implicitCheckHeader("etc/.wh.old", tar.TypeReg, 0)}, "etc"}} {
		t.Run(tc.name, func(t *testing.T) {
			a := newCheckDirectoryDefaults()
			a.headers(tc.base)
			a.headers(tc.change)
			b := newCheckDirectoryDefaults()
			b.headers(tc.base)
			b.headers(tc.change)
			b.directory[tc.which].mtime = 123456
			diff := compareCheckDirectory(a, b, nil, checkIgnore{})
			if len(diff) != 0 {
				t.Fatalf("legitimate import-time directory mtime rejected: %v", diff)
			}
		})
	}
}

func TestExplicitDirectoryTimesRemainChecked(t *testing.T) {
	for _, tc := range []struct {
		name         string
		base, change []*tar.Header
	}{{"explicit same layer", nil, []*tar.Header{implicitCheckHeader("etc/", tar.TypeDir, 10), implicitCheckHeader("etc/tool", tar.TypeReg, 20)}}, {"explicit after implicit", []*tar.Header{implicitCheckHeader("etc/tool", tar.TypeReg, 20)}, []*tar.Header{implicitCheckHeader("etc/", tar.TypeDir, 10)}}, {"unrelated explicit directory", []*tar.Header{implicitCheckHeader("etc/", tar.TypeDir, 10), implicitCheckHeader("var/", tar.TypeDir, 10)}, []*tar.Header{implicitCheckHeader("var/tool", tar.TypeReg, 20)}}} {
		t.Run(tc.name, func(t *testing.T) {
			a := newCheckDirectoryDefaults()
			a.headers(tc.base)
			a.headers(tc.change)
			b := newCheckDirectoryDefaults()
			b.headers(tc.base)
			b.headers(tc.change)
			b.directory["etc"].mtime = 123456
			diff := compareCheckDirectory(a, b, nil, checkIgnore{})
			if len(diff) != 1 || !strings.Contains(diff[0], "mtime") {
				t.Fatalf("explicit or unchanged directory mtime corruption missed: %v", diff)
			}
		})
	}
}

func TestImplicitDirectoryOtherChecksRemain(t *testing.T) {
	for _, field := range []string{"uid", "mode", "filetime", "extra"} {
		t.Run(field, func(t *testing.T) {
			a := newCheckDirectoryDefaults()
			a.headers([]*tar.Header{implicitCheckHeader("etc/tool", tar.TypeReg, 20)})
			b := newCheckDirectoryDefaults()
			b.headers([]*tar.Header{implicitCheckHeader("etc/tool", tar.TypeReg, 20)})
			d := b.directory["etc"]
			switch field {
			case "uid":
				d.uid = 1
			case "mode":
				d.mode = 0o700
			case "filetime":
				f := d.file["tool"]
				f.mtime = 99
				d.file["tool"] = f
			case "extra":
				b.headers([]*tar.Header{implicitCheckHeader("etc/extra", tar.TypeReg, 20)})
			}
			if diff := compareCheckDirectory(a, b, nil, checkIgnore{}); len(diff) == 0 {
				t.Fatal("non-directory-time corruption was ignored")
			}
		})
	}
}

func TestRootDirectoryHeaderTerminates(t *testing.T) {
	done := make(chan struct{})
	go func() {
		c := newCheckDirectoryDefaults()
		c.headers([]*tar.Header{implicitCheckHeader("/", tar.TypeDir, 10)})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("absolute root parent traversal did not terminate")
	}
}

func TestImplicitDirectorySize(t *testing.T) {
	c := newCheckDirectoryDefaults()
	header := implicitCheckHeader("etc/nested/payload", tar.TypeReg, 20)
	header.Size = 7
	c.headers([]*tar.Header{header})
	if c.directory["etc"].size != 0 || c.directory["etc"].directory["nested"].size != 0 {
		t.Fatal("implicit directories inherited file size")
	}
	if c.directory["etc"].directory["nested"].file["payload"].size != 7 {
		t.Fatal("file size constraint lost")
	}
}
