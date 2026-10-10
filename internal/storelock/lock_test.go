package storelock

import "testing"

func TestExclusiveLockAndReopen(t *testing.T) {
	dir := t.TempDir()
	one, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	if two, err := Acquire(dir); err == nil {
		two.Close()
		t.Fatal("two writers acquired the same directory")
	}
	one.Close()
	three, err := Acquire(dir)
	if err != nil {
		t.Fatal("stale lock blocked restart", err)
	}
	three.Close()
}
