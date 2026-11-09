package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A client file that does not exist is named with the folder that holds it to
// create, because AIGW creates neither. A file inside home is named as ~/…; a
// file outside it is named by its own name, so a local check exposes no
// private path.
func TestMissingTargetFixNamesTheFileAndItsFolder(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	inside := filepath.Join(home, "work", "config.toml")
	if got, want := MissingTargetFix(inside, home), "Create ~/work/config.toml and the folder that holds it, or choose another path, then run the same command again."; got != want {
		t.Fatalf("inside home = %q, want %q", got, want)
	}
	outside := filepath.Join(t.TempDir(), "work", "config.toml")
	got := MissingTargetFix(outside, home)
	if got != "Create config.toml and the folder that holds it, or choose another path, then run the same command again." {
		t.Fatalf("outside home = %q", got)
	}
	if strings.Contains(got, filepath.Dir(outside)) {
		t.Fatalf("outside home names its private folder: %q", got)
	}
}

// A regular file that stands where the folder is needed is named to move, on
// Windows too, where such a path reads as missing, and a symbolic link that
// leads nowhere is named to remove, because a create through it fails with
// "file exists".
func TestMissingTargetFixNamesWhatIsInTheWay(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := MissingTargetFix(filepath.Join(file, "config.toml"), root), "Move the file ~/file, then create ~/file/config.toml and the folder that holds it, or choose another path, then run the same command again."; got != want {
		t.Fatalf("a file in the way = %q, want %q", got, want)
	}
	link := filepath.Join(root, "dlink")
	if err := os.Symlink(filepath.Join(root, "nowhere"), link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	got := MissingTargetFix(filepath.Join(link, "config.toml"), root)
	if want := "Remove the link ~/dlink, then create ~/dlink/config.toml and the folder that holds it, or choose another path, then run the same command again."; got != want {
		t.Fatalf("a link in the way = %q, want %q", got, want)
	}
}
