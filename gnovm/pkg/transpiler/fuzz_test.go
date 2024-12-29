package transpiler

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func FuzzTranspiling(f *testing.F) {
	breakRoot := filepath.Join("gnolang", "gno")
	_, thisFile, _, _ := runtime.Caller(0)
	index := strings.Index(thisFile, breakRoot)
	rootPath := thisFile[:index+len(breakRoot)]
	examplesDir := filepath.Join(rootPath, "examples")
	ffs := os.DirFS(examplesDir)
	seedGnoFiles := make([]string, 0, 100)
	fs.WalkDir(ffs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			panic(err)
		}
		if !strings.HasSuffix(path, ".gno") {
			return nil
		}
		seedGnoFiles = append(seedGnoFiles, filepath.Join(examplesDir, path))
		file, err := ffs.Open(path)
		if err != nil {
			panic(err)
		}
		blob, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			panic(err)
		}
		f.Add(blob)
		return nil
	})

	// 1. Derive the seeds from our seedGnoFiles.
	f.Fuzz(func(t *testing.T, gnoSourceCode []byte) {
		// Add timings to ensure that if transpiling takes a long time
		// to run, that we report this as problematic.
		doneCh := make(chan bool, 1)
		readyCh := make(chan bool)
		go func() {
			close(readyCh)
			_, _ = Transpile(string(gnoSourceCode), "gno", "in.gno")
			doneCh <- true
			close(doneCh)
		}()

		<-readyCh

		select {
		case <-time.After(3 * time.Second):
			t.Fatal("took more than 3 seconds to transpile")
		case <-doneCh:
		}
	})
}
