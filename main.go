package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func git(a ...string) (string, error) {
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("git", a...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		lines := strings.Split(stderr.String(), "\n")
		return "", errors.New(lines[0])
	}
	return stdout.String(), nil
}

type version uint

const (
	patch version = iota + 1
	minor
	major
)

type semver struct {
	major      int
	minor      int
	patch      int
	prerelease string
	build      string
}

var errInvalidSemver = errors.New("invalid semver")

func parseSemver(s string) (semver, error) {
	var sem semver

	if s[0] == 'v' {
		s = string(s[1:])
	}

	parts := strings.SplitN(s, ".", 3)

	var tail string

	for i, part := range parts {
		if i == 2 {
			for j, r := range part {
				if !(r >= '0' && r <= '9') {
					tail = part[j:]
					part = part[:j]
					break
				}
			}
		}

		n, err := strconv.ParseInt(part, 10, 64)

		if err != nil {
			return sem, err
		}

		switch i {
		case 0:
			sem.major = int(n)
		case 1:
			sem.minor = int(n)
		case 2:
			sem.patch = int(n)
		}
	}

	if tail == "" {
		return sem, nil
	}

	if tail[0] != '-' && tail[0] != '+' {
		return sem, errInvalidSemver
	}

	prebuf := make([]rune, 0)
	buildbuf := make([]rune, 0)

	var buf *[]rune

	for _, r := range tail {
		if r == '-' {
			buf = &prebuf
			continue
		}

		if r == '+' {
			buf = &buildbuf
			continue
		}
		(*buf) = append((*buf), r)
	}

	sem.prerelease = string(prebuf)
	sem.build = string(buildbuf)

	return sem, nil
}

func (sem *semver) bump(v version) {
	sem.prerelease = ""
	sem.build = ""

	switch v {
	case patch:
		sem.patch++
	case minor:
		sem.patch = 0
		sem.minor++
	case major:
		sem.patch = 0
		sem.minor = 0
		sem.major++
	}
}

func (sem *semver) String() string {
	s := fmt.Sprintf("v%d.%d.%d", sem.major, sem.minor, sem.patch)

	if sem.prerelease != "" {
		s += "-" + sem.prerelease
	}
	if sem.build != "" {
		s += "+" + sem.build
	}
	return s
}

func openInEditor(name string) error {
	editor := os.Getenv("EDITOR")

	if editor == "" {
		return errors.New("EDITOR not set")
	}

	cmd := exec.Command(editor, name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// gitshortlog returns a handle to the file containing the output of
// 'git shortlog revrange'.
func gitshortlog(revrange string) (*os.File, error) {
	f, err := os.CreateTemp("", "release-changelog-*")

	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer

	cmd := exec.Command("git", "shortlog", revrange)
	cmd.Stdout = f
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		lines := strings.Split(stderr.String(), "\n")

		return nil, errors.New(lines[0])
	}

	f.Seek(0, io.SeekStart)
	return f, nil
}

func gittag(tag, tagfile string) error {
	cmd := exec.Command("git", "tag", "-a", tag, "-eF", tagfile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

var notesPreamble = `# Enter notes about the release below. These would provide a high-level overview
# of what's in the release. Lines starting with '#' will be ignored, and the
# output of 'git shortlog' will be appended to the bottom.
`

func release(next version, info bool, prerelease string) (semver, error) {
	var sem semver

	f, err := os.CreateTemp("", "release-*")

	if err != nil {
		return sem, err
	}

	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	io.WriteString(f, notesPreamble)

	if err := openInEditor(f.Name()); err != nil {
		return sem, err
	}

	f.Seek(0, io.SeekStart)

	sc := bufio.NewScanner(f)

	var buf bytes.Buffer

	for sc.Scan() {
		line := sc.Text()

		if len(line) > 0 && line[0] == '#' {
			continue
		}
		fmt.Fprintln(&buf, line)
	}

	if err := sc.Err(); err != nil {
		return sem, err
	}

	f.Truncate(0)
	f.Seek(0, io.SeekStart)

	io.Copy(f, &buf)

	revrange := "HEAD"

	prev, err := git("describe", "--abbrev=0")

	if err == nil {
		prev = strings.TrimSuffix(prev, "\n")

		sem, err = parseSemver(prev)

		if err != nil {
			return sem, err
		}
		revrange = prev + "..HEAD"
	}

	sem.bump(next)
	sem.prerelease = prerelease

	if info {
		sem.build, err = git("log", "-n", "1", "--format=%h")

		if len(sem.build) > 0 {
			// Drop trailing newline.
			sem.build = sem.build[:len(sem.build)-1]
		}

		if err != nil {
			return sem, err
		}
	}

	changelog, err := gitshortlog(revrange)

	if err != nil {
		return sem, err
	}

	defer func() {
		changelog.Close()
		os.Remove(changelog.Name())
	}()

	fmt.Fprintf(f, "\nChangelog:\n\n")
	io.Copy(f, changelog)

	tag := sem.String()

	if err := gittag(tag, f.Name()); err != nil {
		return sem, err
	}

	if _, err := git("archive", "-o", tag + ".tar.gz", tag); err != nil {
		return sem, err
	}
	return sem, nil
}

func main() {
	argv0 := os.Args[0]

	var info bool

	fs := flag.NewFlagSet(argv0, flag.ExitOnError)
	fs.BoolVar(&info, "info", false, "include build metadata")
	fs.Parse(os.Args[1:])

	args := fs.Args()

	if len(args) == 0 {
		fmt.Printf("usage: %s [-info] <major|minor|patch> [pre-release]\n", argv0)
		os.Exit(1)
	}

	var v version

	switch args[0] {
	case "major":
		v = major
	case "minor":
		v = minor
	case "patch":
		v = patch
	default:
		fmt.Fprintf(os.Stderr, "%s: unknown release version %q\n", argv0, args[0])
		os.Exit(1)
	}

	args = args[1:]

	var prerelease string

	if len(args) > 0 {
		prerelease = args[0]
	}

	sem, err := release(v, info, prerelease)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", argv0, err)
		os.Exit(1)
	}
	fmt.Println(sem.String())
}
