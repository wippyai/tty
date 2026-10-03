// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/creack/pty"
)

// The corpora in testdata/ are committed. They are regenerated with
//
//	go test ./vt/conformance -run TestRecordCorpora -update-corpora
//
// which runs real programs in a pty (80x24, TERM=xterm-256color) and writes the
// raw output bytes. Programs that are not installed are skipped.
var updateCorpora = flag.Bool("update-corpora", false, "re-record the real-program corpora and regenerate the synthetic ones")

const (
	corpusCols    = 80
	corpusRows    = 24
	corpusMaxSize = 200 << 10
)

type keystroke struct {
	after time.Duration
	keys  string
}

type recording struct {
	file     string
	argv     []string
	dir      string
	keys     []keystroke
	maxWait  time.Duration
	killOnly bool // the program does not exit by itself; stop it after the keys
}

func record(t *testing.T, r recording) []byte {
	t.Helper()
	path, err := exec.LookPath(r.argv[0])
	if err != nil {
		t.Skipf("%s not installed", r.argv[0])
	}
	cmd := exec.Command(path, r.argv[1:]...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "LANG=C.UTF-8", "LC_ALL=C.UTF-8",
		"COLUMNS=80", "LINES=24", "GIT_PAGER=cat", "HTOP_NO_CONFIG=1")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: corpusRows, Cols: corpusCols})
	if err != nil {
		t.Fatalf("start %v: %v", r.argv, err)
	}
	defer ptmx.Close()

	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&out, ptmx)
		close(done)
	}()
	for _, k := range r.keys {
		time.Sleep(k.after)
		ptmx.Write([]byte(k.keys))
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()
	select {
	case <-exited:
	case <-time.After(r.maxWait):
		cmd.Process.Kill()
		<-exited
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	if out.Len() == 0 {
		t.Fatalf("%v produced no output", r.argv)
	}
	if out.Len() > corpusMaxSize {
		t.Fatalf("%v produced %d bytes, limit %d", r.argv, out.Len(), corpusMaxSize)
	}
	return out.Bytes()
}

func TestRecordCorpora(t *testing.T) {
	if !*updateCorpora {
		t.Skip("run with -update-corpora to re-record")
	}
	work := filepath.Join("testdata", ".work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(work)
	var sample bytes.Buffer
	for i := 1; i <= 120; i++ {
		fmt.Fprintf(&sample, "line %d the quick brown fox jumps over the lazy dog\n", i)
	}
	sampleFile := filepath.Join(work, "sample.txt")
	if err := os.WriteFile(sampleFile, sample.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	editFile := filepath.Join(work, "edit.txt")
	if err := os.WriteFile(editFile, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lsDir := filepath.Join(work, "ls")
	fixtureTree(t, lsDir)
	gitDir := filepath.Join(work, "repo")
	fixtureRepo(t, gitDir)

	ms := time.Millisecond
	recs := []recording{
		{file: "ls-color.vt", argv: []string{"sh", "-c", "ls --color=always -lA --time-style=long-iso; ls --color=always -F"}, dir: lsDir, maxWait: 5 * time.Second},
		{file: "git-log-graph.vt", argv: []string{"git", "--no-pager", "log", "--graph", "--color=always", "--decorate", "--all", "--oneline"}, dir: gitDir, maxWait: 5 * time.Second},
		{
			file: "vim-edit.vt", argv: []string{"vim", "-u", "NONE", "-N", "--noplugin", "-n", editFile}, maxWait: 8 * time.Second,
			keys: []keystroke{
				{600 * ms, "ggihello "}, {150 * ms, "\x1b"}, {150 * ms, "jddo\x1b"}, {150 * ms, "ithird line with some text"},
				{150 * ms, "\x1b"}, {150 * ms, ":set nu\r"}, {200 * ms, "/gamma\r"}, {200 * ms, ":wq\r"},
			},
		},
		{
			file: "less-file.vt", argv: []string{"less", "-R", "-+F", "-X", sampleFile}, maxWait: 8 * time.Second,
			keys: []keystroke{{500 * ms, " "}, {200 * ms, "j"}, {200 * ms, "j"}, {200 * ms, "G"}, {300 * ms, "q"}},
		},
		{
			file: "htop-1s.vt", argv: []string{"htop", "-d", "10"}, maxWait: 6 * time.Second, killOnly: true,
			keys: []keystroke{{1200 * ms, "q"}},
		},
	}
	for _, r := range recs {
		t.Run(r.file, func(t *testing.T) {
			data := record(t, r)
			if err := os.WriteFile(filepath.Join("testdata", r.file), data, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %d bytes", r.file, len(data))
		})
	}
}

// fixtureTree builds a directory with the file kinds ls colors differently.
func fixtureTree(t *testing.T, dir string) {
	t.Helper()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	fixedTime := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	must(os.MkdirAll(filepath.Join(dir, "src", "nested"), 0o755))
	must(os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	for name, mode := range map[string]os.FileMode{
		"README.md": 0o644, "main.go": 0o644, "run.sh": 0o755, "archive.tar.gz": 0o644, "image.png": 0o644,
		"data.json": 0o644, "Makefile": 0o644, "notes.txt": 0o600, "release.zip": 0o644, "tool": 0o755,
	} {
		path := filepath.Join(dir, name)
		must(os.WriteFile(path, bytes.Repeat([]byte("x"), 100+len(name)*37), mode))
		must(os.Chtimes(path, fixedTime, fixedTime))
	}
	must(os.Symlink("main.go", filepath.Join(dir, "link-to-main")))
	must(os.Symlink("missing-target", filepath.Join(dir, "broken-link")))
	for _, d := range []string{"src", "docs", "src/nested"} {
		must(os.Chtimes(filepath.Join(dir, d), fixedTime, fixedTime))
	}
}

// fixtureRepo builds a repository whose history has branches and merges so
// that git log --graph draws its full set of connectors.
func fixtureRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	n := 0
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		n++
		stamp := fmt.Sprintf("2026-01-%02dT10:00:00Z", n%27+1)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=T", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=T",
			"GIT_COMMITTER_EMAIL=t@example.com", "GIT_AUTHOR_DATE="+stamp, "GIT_COMMITTER_DATE="+stamp,
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	file := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	file("a.txt", "a\n")
	git("add", "-A")
	git("commit", "-q", "-m", "initial commit")
	for _, br := range []string{"feature-one", "feature-two", "bugfix"} {
		git("checkout", "-q", "-b", br, "main")
		for i := 1; i <= 3; i++ {
			file(br+".txt", fmt.Sprintf("%s %d\n", br, i))
			git("add", "-A")
			git("commit", "-q", "-m", fmt.Sprintf("%s: step %d of the change with a longer message to wrap the line", br, i))
		}
		git("checkout", "-q", "main")
		file("main-"+br+".txt", br+"\n")
		git("add", "-A")
		git("commit", "-q", "-m", "main: work while "+br+" is open")
		git("merge", "-q", "--no-ff", "-m", "Merge branch '"+br+"'", br)
	}
	git("tag", "v1.0.0")
}
