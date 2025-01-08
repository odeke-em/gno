package gnolang

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
)

func FuzzConvertUntypedBigdecToFloat(f *testing.F) {
	// 1. Firstly add seeds.
	seeds := []string{
		"-100000",
		"100000",
		"0",
	}

	check := new(apd.Decimal)
	for _, seed := range seeds {
		if check.UnmarshalText([]byte(seed)) == nil {
			f.Add(seed)
		}
	}

	f.Fuzz(func(t *testing.T, apdStr string) {
		switch {
		case strings.HasPrefix(apdStr, ".-"):
			return
		}

		v := new(apd.Decimal)
		if err := v.UnmarshalText([]byte(apdStr)); err != nil {
			return
		}
		if _, err := v.Float64(); err != nil {
			return
		}

		bd := BigdecValue{
			V: v,
		}
		dst := new(TypedValue)
		typ := Float64Type
		ConvertUntypedBigdecTo(dst, bd, typ)
	})
}

func FuzzParseFile(f *testing.F) {
	// 1. Add the corpra.
	parseFileDir := filepath.Join("testdata", "corpra", "parsefile")
	paths, err := filepath.Glob(filepath.Join(parseFileDir, "*.go"))
	if err != nil {
		f.Fatal(err)
	}

	// Also load in files from gno/gnovm/tests/files
	_, curFile, _, _ := runtime.Caller(0)
	curFileDir := filepath.Dir(curFile)
	gnovmTestFilesDir, err := filepath.Abs(filepath.Join(curFileDir, "..", "..", "tests", "files"))
	if err != nil {
		f.Fatal(err)
	}
	globGnoTestFiles := filepath.Join(gnovmTestFilesDir, "*.gno")
	gnoTestFiles, err := filepath.Glob(globGnoTestFiles)
	if err != nil {
		f.Fatal(err)
	}
	if len(gnoTestFiles) == 0 {
		f.Fatalf("no files found from globbing %q", globGnoTestFiles)
	}
	paths = append(paths, gnoTestFiles...)

	for _, path := range paths {
		blob, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(blob))
	}

	// 2. Now run the fuzzer.
	f.Fuzz(func(t *testing.T, goFileContents string) {
		_, _ = ParseFile("a.go", goFileContents)
	})
}

func FuzzBitShiftingOverflow(f *testing.F) {
	if testing.Short() {
		f.Skip("Skippingin -short mode")
	}

	// 1. Add the seeds.
	for _, tt := range bitShiftingTests {
		f.Add(tt.source)
	}

	// 2. Run the fuzzer.
	f.Fuzz(func(t *testing.T, srcCode string) {
		switch strings.TrimSpace(srcCode) {
		case `package test\nfunc main(){const c1=1<8\nmain()\n\t1\t}`:
			return
		default:
		}

		defer func() {
			r := recover()
			if r == nil {
				return
			}

			tempDir := t.TempDir()
			mainGo := filepath.Join(tempDir, "main.go")
			if err := os.WriteFile(mainGo, []byte(srcCode), 0755); err != nil {
				panic(err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "go", "run", mainGo)
			if err := cmd.Run(); err == nil {
				panic(fmt.Sprintf("Code runs successfully in Go but not Gno: discrepancy\n%s\n\n%s", srcCode, r))
			}

			if false {
				// If the code parses alright in Go but not in Gno, that's a problem.
				fset := token.NewFileSet()
				_, err := parser.ParseFile(fset, "src.go", srcCode, 0)
				if err == nil {
					panic(fmt.Sprintf("Code parses in Go but not Gno: discrepancy\n%s\n\n%s", srcCode, r))
				}

				rs := fmt.Sprintf("%s", r)
				switch {
				case strings.Contains(rs, "constant overflows"):
					return
				case strings.Contains(rs, "bigint overflows"):
					return
				case strings.Contains(rs, "expected 'package'"):
					return
				case strings.Contains(rs, "not terminated"):
					return
				default:
					panic(r)
				}
			}
		}()

		m := NewMachine("test", nil)
		ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
		m.SetContext(ctx)
		n := MustParseFile("main.go", srcCode)
		m.RunFiles(n)
		m.RunMain()
	})
}

func FuzzMemPkgTypeCheck(f *testing.F) {
	if testing.Short() {
		f.Skip("Skippingin -short mode")
	}

	// 1. Add the seeds.
	for _, tt := range memPkgTypeCheckTests {
		f.Add(tt.source)
	}

}
