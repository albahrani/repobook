# Windows Deadlock Fix for Repobook

## Problem

When running `repobook.exe c:\workspace\projects\` on Windows, the application crashes with:
```
fatal error: all goroutines are asleep - deadlock!
```

The deadlock occurs in the file watcher (`fsnotify`) during initialization.

## Root Cause

The issue is in `/internal/watch/watcher.go` in the `NewWatcher` function:

1. The function calls `filepath.WalkDir` to recursively add all directories to the file watcher
2. For each directory, it calls `w.Add(p)` to register it with fsnotify
3. **Problem**: On Windows, fsnotify sends events synchronously during `Add()` calls
4. These events try to write to the `w.w.Events` channel
5. **But**: The event loop that reads from this channel (`ww.loop()`) doesn't start until **after** all directories are added
6. This causes a deadlock: `Add()` is blocked trying to send events, waiting for someone to read them

## Solution

Start the event loop **before** walking the directory tree, so events can be consumed as they are generated.

## Changes Made

**File**: `internal/watch/watcher.go`

**Before** (lines 28-62):
```go
ww := &Watcher{rootAbs: rootAbs, ignore: ig, hub: hub, w: w, done: make(chan struct{})}

// Watch all directories initially (fsnotify is not recursive).
err = filepath.WalkDir(rootAbs, func(p string, d fs.DirEntry, walkErr error) error {
    // ... directory walking code ...
})
if err != nil {
    _ = w.Close()
    return nil, err
}

go ww.loop()  // ← Event loop starts AFTER adding all watches
return ww, nil
```

**After** (lines 28-66):
```go
ww := &Watcher{rootAbs: rootAbs, ignore: ig, hub: hub, w: w, done: make(chan struct{})}

// Start the event loop before adding watches to prevent deadlock on Windows
// where fsnotify may send events synchronously during Add()
go ww.loop()  // ← Event loop starts BEFORE adding watches

// Watch all directories initially (fsnotify is not recursive).
err = filepath.WalkDir(rootAbs, func(p string, d fs.DirEntry, walkErr error) error {
    // ... directory walking code ...
})
if err != nil {
    _ = w.Close()
    return nil, err
}

return ww, nil
```

## How to Build and Test

On your Windows machine:

1. Apply the changes to `internal/watch/watcher.go` (move `go ww.loop()` before `filepath.WalkDir`)
2. Build the project:
   ```cmd
   go build ./cmd/repobook
   ```
3. Test with the problematic path:
   ```cmd
   repobook.exe c:\workspace\projects\
   ```

The application should now start successfully without deadlocking.

## Additional Notes

- This fix is safe for all platforms (Linux, macOS, Windows)
- Starting the event loop early doesn't cause any issues because the watcher object is fully initialized before any events can trigger the `handle()` method
- The `done` channel is already created, so closing the watcher will properly shut down the loop even if an error occurs during WalkDir

## Testing Checklist

- [ ] Application starts without deadlock on Windows
- [ ] File watching works correctly (make changes to markdown files and verify browser updates)
- [ ] Application works on large directory trees (e.g., C:\workspace\projects\)
- [ ] No regressions on Linux/macOS (if you have access to test)
