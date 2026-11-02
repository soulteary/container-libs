package storage

import (
	"archive/tar"
	"strings"
	"testing"
	"time"
)

func TestImplicitDirectorySize(t *testing.T) {
	c := newCheckDirectoryDefaults()
	header := &tar.Header{Name: "etc/nested/payload", Typeflag: tar.TypeReg, Mode: 0o755, Size: 7, ModTime: time.Unix(20, 0)}
	c.headers([]*tar.Header{header})
	if c.directory["etc"].size != 0 || c.directory["etc"].directory["nested"].size != 0 {
		t.Fatal("implicit directories inherited file size")
	}
	if c.directory["etc"].directory["nested"].file["payload"].size != 7 {
		t.Fatal("file size constraint lost")
	}
}

func TestImplicitDirectoryConstraintsRemainChecked(t *testing.T) {
	for _, field := range []string{"mtime", "uid", "gid", "mode", "file size"} {
		t.Run(field, func(t *testing.T) {
			header := &tar.Header{Name: "etc/payload", Typeflag: tar.TypeReg, Mode: 0o755, Size: 7, ModTime: time.Unix(20, 0)}
			expected := newCheckDirectoryDefaults()
			expected.headers([]*tar.Header{header})
			actual := newCheckDirectoryDefaults()
			actual.headers([]*tar.Header{header})
			switch field {
			case "mtime":
				actual.directory["etc"].mtime++
			case "uid":
				actual.directory["etc"].uid++
			case "gid":
				actual.directory["etc"].gid++
			case "mode":
				actual.directory["etc"].mode = 0o700
			case "file size":
				f := actual.directory["etc"].file["payload"]
				f.size++
				actual.directory["etc"].file["payload"] = f
			}
			diffs := compareCheckDirectory(expected, actual, nil, checkIgnore{})
			if len(diffs) == 0 {
				t.Fatal("modified attribute not detected")
			}
			if field == "mtime" && !strings.Contains(strings.Join(diffs, " "), "mtime") {
				t.Fatal("directory mtime constraint lost")
			}
		})
	}
}
