# RAW SIVR

A fast desktop app for sorting a shoot: go through your photos and videos with the keyboard, keep the good ones, and clean up the rest. RAW and JPG files with the same name are treated as one photo, so you never hunt for the matching RAW by hand.

Built with Go and [Wails](https://wails.io) (React frontend). Developed and tested on Windows with Sony cameras (ARW + JPG, XAVC video). Linux and macOS builds are set up but have not been tested yet; see [Platform status](#platform-status).

## What it does

**The usual manual workflow**

1. Copy the card to the computer.
2. Copy the JPGs somewhere, look through them, move the good ones to a folder.
3. Find the RAW file for each good JPG by name and copy those to another folder.
4. Delete the rest.

**With RAW SIVR**

1. Import the card (or open a folder you already copied).
2. Press `P` on the photos you want and `X` on the ones you don't.
3. Click **Apply**. Picked JPGs go to `selected_jpg`, their RAWs to `selected_raw`, picked videos to `selected_video`, and rejects go to the Recycle Bin if you choose that.

## Features

- **RAW + JPG pairing.** `DSC01234.JPG` and `DSC01234.ARW` are one photo. RAW-only shots are shown using the preview embedded in the RAW file.
- **Grid and single view** with a filmstrip, filters (all, unmarked, picked, rejected) and counters.
- **100% zoom** for checking focus. The zoom position is kept while you step through photos.
- **Side-by-side compare** of two to four photos with a shared zoom position.
- **Burst grouping.** Photos shot within two seconds of each other are grouped. One key keeps the current frame and rejects the rest of the burst.
- **Shooting info**: shutter, aperture, ISO, focal length, camera and lens.
- **Video.** Clips play as a 1080p preview that is built in the background and cached. On Windows, the camera original can also be played through an embedded [mpv](https://mpv.io) player.
- **Import** from a camera card or any folder into a dated folder such as `2026-10-26_Moon_night`. Copies are verified and the card is never changed.
- **Orphan RAW finder.** Finds RAW files whose JPG no longer exists anywhere in a folder tree, for cleaning up older shoots.
- **Safe by default.** See [Safety](#safety).

## Requirements

| | Needed for | Notes |
|---|---|---|
| [ffmpeg](https://ffmpeg.org) on your `PATH` | Video thumbnails and previews | Photos work without it |
| [mpv](https://mpv.io) | Playing original video files (Windows only for now) | Optional. Found on `PATH` or in `C:\Program Files\MPV Player` |
| WebView2 runtime | The app window on Windows | Already part of Windows 11 |

On Windows, both tools can be installed with winget:

```
winget install Gyan.FFmpeg
winget install shinchiro.mpv
```

## Build from source

You need [Go](https://go.dev/dl/) 1.26 or newer, [Node.js](https://nodejs.org) 20 or newer, and the Wails CLI:

```
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
```

Then, in the project folder:

```
wails build
```

The app is written to `build/bin/`. During development, `wails dev` runs it with live reload of the frontend.

On Linux, install the GTK and WebKit development packages first (Ubuntu 24.04: `libgtk-3-dev libwebkit2gtk-4.1-dev`) and build with `wails build -tags webkit2_41`.

## Using it

### Apply modes

| Mode | Picks | Rejects | Unmarked |
|---|---|---|---|
| **Copy picks** (default) | Copied to the selected folders | Not touched | Not touched |
| **Move picks and clean up** | Moved to the selected folders | Sent to the Recycle Bin | Left in place |
| **Clean up rejects only** | Not touched | Sent to the Recycle Bin | Left in place |

The selected folders are created inside the shoot folder. Their names can be changed in Settings.

### Keyboard

| Key | Action |
|---|---|
| Arrow keys | Move between photos |
| `Enter` | Open the single view, or go back to the grid |
| `Esc` | Back to the grid |
| `P` / `X` / `U` | Pick / reject / clear the mark |
| `Z` | 100% zoom on and off (or click the photo; drag to pan) |
| `C` | Compare two photos; `2` `3` `4` set the number of panes |
| `B` | Compare the whole burst |
| `K` | Keep this frame, reject the rest of its burst |
| `I` | Show or hide the shooting info |
| `Space` | Play or pause a video |
| `O` | Switch a video between preview and original |

The letter keys can be changed in Settings.

## Safety

- **Nothing is deleted permanently.** Rejects and orphans go to the Recycle Bin (the trash on Linux and macOS).
- **Copy is the default.** Your original folder is only changed if you choose the move or clean-up mode.
- **You see the plan first.** Apply shows how many files will be copied, moved or recycled, and how much space that frees, before anything happens.
- **No overwriting.** If a different file with the same name is already in the destination, it is skipped and reported.
- **Verified copies.** Every copied file is read back and compared with the source.
- **Undo.** The last Apply can be undone: copies are removed and moved files are put back. Recycled files have to be restored from the Recycle Bin.
- **Import never touches the card.**

## Where things are stored

| What | Where |
|---|---|
| Pick / reject marks and the undo journal | `.rawsivr` folder inside each shoot folder |
| Settings | `%APPDATA%\rawsivr\config.json` (the user config folder on other systems) |
| Thumbnails and video previews | `%LOCALAPPDATA%\rawsivr` (the user cache folder on other systems); safe to delete |

## Platform status

| Platform | Status |
|---|---|
| Windows 11 | Developed and tested here |
| Linux | Compiles; built by CI. Not tested on a real machine. Original video playback is not available, previews are |
| macOS | Compiles; built by CI. Not tested on a real machine. Original video playback is not available, previews are |

Known limits:

- RAW support is Sony ARW only. Other formats need their extension added in `internal/scan` and a check that the embedded preview is found.
- Video previews are built at about real-time speed for high-bitrate camera files, because decoding the original is the slow part.
- Original 8K playback has not been tested.
- Burst grouping is by time only (a fixed two-second gap).

## Project layout

```
main.go, app.go, keys.go   Wails app and the methods the UI calls
cmd/rawsivr-cli/         Command-line tool for the core engine (scan, mark, plan, apply, undo)
internal/scan/             Folder walk, RAW + JPG pairing, video sidecars
internal/store/            Pick / reject marks
internal/plan/             Works out what Apply will do, without touching the disk
internal/execute/          Copy, move, recycle, journal and undo
internal/trash/            Recycle Bin / trash per platform
internal/exif/             EXIF reading and embedded RAW previews
internal/thumb/            Thumbnail cache
internal/burst/            Burst grouping
internal/importer/         Import from a card or folder
internal/orphans/          Orphan RAW finder
internal/media/            Local HTTP server that feeds images and video to the UI
internal/video/            Video previews (ffmpeg) and the embedded mpv player
frontend/                  React + TypeScript + Tailwind UI
```

## Development

```
go test ./...          # run `wails build` once first: the app package embeds frontend/dist
go vet ./...
cd frontend && npx tsc --noEmit
```

Two tests read real camera files when you point them at a folder: set `RAWSIVR_SAMPLES` to a folder containing JPG and ARW files.

Contributions are welcome, especially testing on Linux and macOS and support for other RAW formats.

## License

[MIT](LICENSE)

RAW SIVR runs ffmpeg and mpv as separate programs when they are installed; they are not part of this project and have their own licenses.
