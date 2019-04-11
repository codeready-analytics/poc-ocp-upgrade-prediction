package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"go.uber.org/zap"
)

var logger, _ = zap.NewDevelopment()
var sugarLogger = logger.Sugar()

// RunCloneShell runs a git clone inside a child shell and clones it as a subdir inside destdir in the workspace style
// of go.
func RunCloneShell(repo, destdir, branch, revision string) string {
	repo = strings.Split(repo, ".git")[0]

	clonePathUrl, err := url.Parse(repo)
	if err != nil {
		sugarLogger.Errorf("%v\n", err)
	}
	clonePath := filepath.Join(destdir, "src", clonePathUrl.Host, clonePathUrl.Path)
	if _, err := os.Stat(destdir); os.IsNotExist(err) {
		errdir := os.Mkdir(destdir, os.ModePerm)
		if errdir != nil {
			sugarLogger.Fatal(errdir)
		}
	}

	// If the path exists there will be no error.
	_, nilIfExists := os.Stat(clonePath)

	if nilIfExists == nil {
		sugarLogger.Infof("A repo with that remote URL already exists at %v in local clones, not cloning again.", clonePath)
		return clonePath
	}

	cmdRun := exec.Command("git", "clone", repo, clonePath, "--branch", branch)
	stdouterr, err := cmdRun.CombinedOutput()

	if err != nil {
		sugarLogger.Error(string(stdouterr))
		sugarLogger.Error(err)
	}

	cmdRun = exec.Command("git", "-C", clonePath, "checkout", revision)
	stdouterr, err = cmdRun.CombinedOutput()

	if err != nil {
		sugarLogger.Error(string(stdouterr))
		sugarLogger.Error(err)
	}

	return filepath.Join(clonePath)
}

// CopyFile copies a file
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

// InstallDependency installs the tracer using the correct package manager.
func InstallDependency(manifestFileName string) {
	pkgStr := "github.com/rootAvish/go-tracey@d6c82cd1b2fb258bcb40afadcf4b1e0538b81492"
	var pkgManagerCommand string
	if filepath.Ext(manifestFileName) == ".toml" {
		// Run dep install command
		pkgManagerCommand = "dep ensure -add " + pkgStr

	} else if filepath.Ext(manifestFileName) == ".yaml" {
		// Run glide install command
		pkgManagerCommand = "glide get " + pkgStr
	} else if filepath.Ext(manifestFileName) == ".json" {
		// Run godeps install command
		pkgManagerCommand = fmt.Sprintf("go get \"%s\";godeps save ./...;", pkgStr)
	} else {
		// There is no "vgo install"
		pkgManagerCommand = ""
	}

	if pkgManagerCommand != "" {
		cmdRun := exec.Command(pkgManagerCommand)
		stdout, err := cmdRun.CombinedOutput()
		if err != nil {
			sugarLogger.Error(string(stdout))
			sugarLogger.Error(err)
		}
	}
}

// ReadFileLines returns an array of strings from a file, just like python.
func ReadFileLines(fn string) ([]string, error) {
	var fileLines []string
	file, err := os.Open(fn)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	// Start reading from the file with a reader.
	reader := bufio.NewReader(file)

	var line string
	for {
		line, err = reader.ReadString('\n')
		// Process the line here.
		fileLines = append(fileLines, line)
		if err != nil {
			break
		}
	}

	if err != io.EOF {
		sugarLogger.Errorf(" > Failed!: %v\n", err)
	}

	return fileLines, nil
}

// ReSubMatchMap matches subgroups of a regex and returns it in a named map. From: https://stackoverflow.com/a/46202939
func ReSubMatchMap(r *regexp.Regexp, str string) map[string]string {
	match := r.FindStringSubmatch(str)
	subMatchMap := make(map[string]string)
	if match == nil {
		return nil
	}
	for i, name := range r.SubexpNames() {
		if i != 0 {
			subMatchMap[name] = match[i]
		}
	}

	return subMatchMap
}

// RunCmdWithWait runs a command us cmd.Wait()
func RunCmdWithWait(cmd *exec.Cmd) (string, string) {
	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()

	var errStdout, errStderr error
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)
	err := cmd.Start()
	if err != nil {
		sugarLogger.Fatalf("cmd.Start() failed with '%s'\n", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
		wg.Done()
	}()

	_, errStderr = io.Copy(stderr, stderrIn)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		sugarLogger.Fatalf("cmd.Run() failed with %s\n", err)
	}
	if errStdout != nil || errStderr != nil {
		sugarLogger.Fatal("failed to capture stdout or stderr\n")
	}
	outStr, errStr := string(stdoutBuf.Bytes()), string(stderrBuf.Bytes())
	return outStr, errStr
}

// WriteStringToFile, writes string s to filepath
func WriteStringToFile(filepath, s string) error {
	fo, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer fo.Close()

	_, err = io.Copy(fo, strings.NewReader(s))
	if err != nil {
		return err
	}

	return nil
}
