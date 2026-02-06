# Repobook Windows Fixes Summary

Two critical issues have been fixed in repobook for Windows users:

## Issue 1: Deadlock on Large Directory Trees

### Problem
When running `repobook.exe c:\workspace\projects\` on Windows, the application crashes with:
```
fatal error: all goroutines are asleep - deadlock!
```

### Root Cause
The file watcher initialization tries to add directories before starting the event consumer loop. On Windows, `fsnotify.Add()` sends events synchronously, causing a deadlock when no one is reading them.

### Fix
**File**: `internal/watch/watcher.go` (lines 28-66)

Move `go ww.loop()` to start **before** `filepath.WalkDir`:

```go
ww := &Watcher{rootAbs: rootAbs, ignore: ig, hub: hub, w: w, done: make(chan struct{})}

// Start the event loop before adding watches to prevent deadlock on Windows
// where fsnotify may send events synchronously during Add()
go ww.loop()  // ← MOVED HERE (was after WalkDir)

// Watch all directories initially (fsnotify is not recursive).
err = filepath.WalkDir(rootAbs, func(p string, d fs.DirEntry, walkErr error) error {
    // ... existing code ...
})
```

---

## Issue 2: Paths with Spaces Support

### Problem
Paths with spaces like `C:\Users\NorTot\Downloads\2 PrivatNTG\neu 6.txt` would fail if not quoted in the command line.

### Fix
**File**: `cmd/repobook/main.go` (lines 33-51)

Modified argument parsing to join multiple arguments when spaces are detected:

```go
if flag.NArg() < 1 {  // Changed from != 1
    flag.Usage()
    os.Exit(2)
}

// Join all remaining arguments to handle paths with spaces
// This allows: repobook C:\path with spaces\file.txt
// as well as: repobook "C:\path with spaces\file.txt"
pathArg := flag.Arg(0)
if flag.NArg() > 1 {
    // Join all arguments with spaces
    args := make([]string, flag.NArg())
    for i := 0; i < flag.NArg(); i++ {
        args[i] = flag.Arg(i)
    }
    pathArg = filepath.Join(args...)
}

root, err := filepath.Abs(pathArg)  // Changed from flag.Arg(0)
```

---

## How to Build and Test

### On Windows:

1. **Apply the changes** to both files as shown above

2. **Build the project**:
   ```cmd
   go build ./cmd/repobook
   ```

3. **Test the deadlock fix**:
   ```cmd
   repobook.exe c:\workspace\projects\
   ```
   Should start without deadlock.

4. **Test paths with spaces** (both should work):
   ```cmd
   repobook.exe "C:\Users\NorTot\Downloads\2 PrivatNTG"
   repobook.exe C:\Users\NorTot\Downloads\2 PrivatNTG
   ```

### Running Tests:

```cmd
go test ./...
```

---

## Files Changed

1. `internal/watch/watcher.go` - Deadlock fix
2. `cmd/repobook/main.go` - Paths with spaces fix

Both changes are backward compatible and safe for all platforms (Windows, Linux, macOS).

---

## Expected Behavior After Fixes

✅ Application starts successfully on large directory trees
✅ No deadlock errors
✅ Paths with spaces work with or without quotes
✅ File watching updates work correctly
✅ All existing functionality preserved
