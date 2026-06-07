# go-mpv

[![Go Reference](https://pkg.go.dev/badge/github.com/xmengnet/go-mpv.svg)](https://pkg.go.dev/github.com/xmengnet/go-mpv)

> Go bindings for [libmpv](https://mpv.io/).

Forked from [gen2brain/go-mpv](https://github.com/gen2brain/go-mpv), focused on CGo with `mpv_node` API support.

## Features

- Full CGo bindings for libmpv
- `mpv_node` API support for complex data structures
- Async command/property operations
- Event system for playback state changes

## Requirements

- Go 1.19+
- libmpv development files

### Install libmpv

```bash
# Debian/Ubuntu
sudo apt install libmpv-dev

# Arch Linux
sudo pacman -S mpv

# macOS
brew install mpv

# Fedora
sudo dnf install mpv-devel
```

## Installation

```bash
go get github.com/xmengnet/go-mpv
```

## Usage

### Basic Example

```go
package main

import (
	"log"

	mpv "github.com/xmengnet/go-mpv"
)

func main() {
	m := mpv.Create()

	// Configure before initialize
	m.SetOptionString("input-default-bindings", "yes")
	m.SetOptionString("input-vo-keyboard", "yes")
	m.SetOption("osc", mpv.FORMAT_FLAG, true)

	if err := m.Initialize(); err != nil {
		log.Fatal(err)
	}

	// Play a file
	if err := m.Command([]string{"loadfile", "video.mp4"}); err != nil {
		log.Fatal(err)
	}

	// Event loop
	for {
		e := m.WaitEvent(10000)
		switch e.Event_Id {
		case mpv.EVENT_FILE_LOADED:
			title := m.GetPropertyString("media-title")
			log.Println("Now playing:", title)
		case mpv.EVENT_SHUTDOWN:
			goto exit
		}
	}

exit:
	m.TerminateDestroy()
}
```

### Stream URL (e.g. Emby/Jellyfin)

```go
m := mpv.Create()

// Set authentication headers
m.SetOptionString("http-header-fields", "X-Emby-Token: YOUR_API_KEY")

// Cache settings for streaming
m.SetOption("cache", mpv.FORMAT_FLAG, true)
m.SetOption("cache-secs", mpv.FORMAT_DOUBLE, 30.0)

m.Initialize()

// Play stream
m.Command([]string{"loadfile", "http://your-server:8096/emby/videos/1234/stream?api_key=xxx"})
```

### Get Metadata

```go
// Simple properties
title := m.GetPropertyString("media-title")
duration, _ := m.GetProperty("duration", mpv.FORMAT_DOUBLE)

// Full metadata (artist, album, etc.)
result, _ := m.GetProperty("metadata", mpv.FORMAT_NODE)
if result != nil {
    metadata := result.(*mpv.Node)
    if m, ok := metadata.Data.(map[string]*mpv.Node); ok {
        if artist, ok := m["artist"]; ok {
            fmt.Println("Artist:", artist.Data)
        }
        if album, ok := m["album"]; ok {
            fmt.Println("Album:", album.Data)
        }
    }
}
```

### Async Commands

```go
// Non-blocking command execution
m.CommandAsync(1, []string{"loadfile", "next-song.mp3"})

// Get property asynchronously
m.GetPropertyAsync("time-pos", 2, mpv.FORMAT_DOUBLE)

// Results come via event loop
for {
    e := m.WaitEvent(10000)
    if e.Event_Id == mpv.EVENT_COMMAND_REPLY && e.Reply_Userdata == 1 {
        // Async command completed
    }
}
```

## API Reference

### Core

| Function | Description |
|----------|-------------|
| `Create()` | Create new mpv instance |
| `Initialize()` | Initialize mpv (call after SetOption) |
| `TerminateDestroy()` | Destroy mpv instance |
| `Command([]string)` | Execute mpv command |
| `CommandString(string)` | Execute command string |

### Properties

| Function | Description |
|----------|-------------|
| `GetProperty(name, format)` | Get property value |
| `GetPropertyString(name)` | Get property as string |
| `SetProperty(name, format, data)` | Set property value |
| `SetPropertyString(name, value)` | Set property as string |
| `ObserveProperty(id, name, format)` | Watch for property changes |

### Events

| Function | Description |
|----------|-------------|
| `WaitEvent(timeout)` | Wait for next event |
| `RequestEvent(event, enable)` | Enable/disable event type |
| `RequestLogMessages(level)` | Enable log messages |

### Event Types

| Event | Description |
|-------|-------------|
| `EVENT_START_FILE` | Playback started |
| `EVENT_FILE_LOADED` | File loaded |
| `EVENT_END_FILE` | Playback ended |
| `EVENT_SEEK` | Seek performed |
| `EVENT_SHUTDOWN` | Player shutting down |
| `EVENT_PROPERTY_CHANGE` | Property value changed |

## Related Projects

- [gen2brain/go-mpv](https://github.com/gen2brain/go-mpv) - Upstream (includes purego implementation)
- [Supersonic](https://github.com/dweymouth/supersonic) - Subsonic client using go-mpv
- [mpv.io](https://mpv.io/) - mpv player documentation

## License

See [LICENSE](LICENSE) file.
