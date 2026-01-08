# cncstats
A replay parser for Command and Conquer: Generals: Zero Hour. Includes mappings of most commands and support for parsing INI files to get unit/building info.

## Features

- **Complete Replay Parsing**: Parse entire replay files and extract all game events
- **Streaming Replay Parsing**: Stream replay events as they're being written to a file (useful for live game monitoring)
- **INI Data Integration**: Parse CNC INI files to get unit, building, upgrade, and power information
- **Web API**: HTTP endpoint for uploading and parsing replay files
- **Command Line Tool**: Process individual replay files locally

## Usage

### Command Line Usage

```bash
# Process a single replay file
./cncstats -local -file replay.rep

# Process with custom INI data path
./cncstats -local -file replay.rep -objdata /path/to/ini/data

# Run web server
./cncstats
```

### Library Usage

#### Complete Replay Parsing

```go
package main

import (
    "os"
    "github.com/bill-rich/cncstats/pkg/bitparse"
    "github.com/bill-rich/cncstats/pkg/iniparse"
    "github.com/bill-rich/cncstats/pkg/zhreplay"
)

func main() {
    // Initialize stores
    objectStore, _ := iniparse.NewObjectStore("./inizh/Data/INI")
    powerStore, _ := iniparse.NewPowerStore("./inizh/Data/INI")
    upgradeStore, _ := iniparse.NewUpgradeStore("./inizh/Data/INI")
    
    // Open replay file
    file, _ := os.Open("replay.rep")
    defer file.Close()
    
    // Create parser
    bp := &bitparse.BitParser{
        Source:       file,
        ObjectStore:  objectStore,
        PowerStore:   powerStore,
        UpgradeStore: upgradeStore,
    }
    
    // Parse complete replay
    replay := zhreplay.NewReplay(bp)
    
    // Access parsed data
    fmt.Printf("Map: %s\n", replay.Header.Metadata.MapFile)
    fmt.Printf("Events: %d\n", len(replay.Body))
}
```

#### Streaming Replay Parsing

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/bill-rich/cncstats/pkg/iniparse"
    "github.com/bill-rich/cncstats/pkg/zhreplay"
)

func main() {
    // Initialize stores
    objectStore, _ := iniparse.NewObjectStore("./inizh/Data/INI")
    powerStore, _ := iniparse.NewPowerStore("./inizh/Data/INI")
    upgradeStore, _ := iniparse.NewUpgradeStore("./inizh/Data/INI")
    
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    // Configure streaming options
    options := &zhreplay.StreamReplayOptions{
        PollInterval: 100 * time.Millisecond,
        MaxWaitTime:  30 * time.Second,
        BufferSize:   100,
    }
    
    // Start streaming
    bodyChan, streamingReplay, err := zhreplay.StreamReplay(
        ctx, "replay.rep", objectStore, powerStore, upgradeStore, options)
    if err != nil {
        log.Fatal(err)
    }
    
    // Print header info
    fmt.Printf("Map: %s\n", streamingReplay.Header.Metadata.MapFile)
    
    // Process events as they arrive
    for chunk := range bodyChan {
        fmt.Printf("Event: %s at time %d\n", chunk.OrderName, chunk.TimeCode)
        
        // Check for EndReplay command
        if chunk.OrderCode == 27 {
            fmt.Println("Game ended")
            break
        }
    }
}
```

### Web API Usage

Start the web server:
```bash
./cncstats
```

Upload a replay file:
```bash
curl -X POST -F "file=@replay.rep" http://localhost:8080/replay
```

## Examples

See `examples/example_streaming.go` for a complete example of streaming replay parsing.

## Useful commands
Filter by playerID, and remove checksum, deselects, and camera movements.
`jq '.Body[] | select(.OrderCode != 1095 and .OrderCode != 1092 and .OrderCode != 1003 and .OrderCode != 2000 and .OrderCode != 1001 and .OrderCode != 1058 and .OrderCode != 1068)' | less`

## Annotated outputs/thoghts
```
{
  "TimeCode": 210,
  "OrderType": 1001,  // Select
  "Number": 2,
  "UniqueOrders": 2,
  "Args": [
    {
      "Type": 2,
      "Count": 1,
      "Args": [
        true
      ]
    },
    {
      "Type": 3,
      "Count": 1,
      "Args": [
        376         // First USA Command Center
      ]
    }
  ]
}
{
  "TimeCode": 240,
  "OrderType": 1047,   // Build
  "Number": 2,
  "UniqueOrders": 1,
  "Args": [
    {
      "Type": 0,
      "Count": 2,
      "Args": [
        135,          // Another dozer (Dozer 2)
        1
      ]
    }
  ]
}

{
  "TimeCode": 412,
  "OrderType": 1001, // Select
  "Number": 2,
  "UniqueOrders": 2,
  "Args": [
    {
      "Type": 2,
      "Count": 1,
      "Args": [
        true
      ]
    },
    {
      "Type": 3,
      "Count": 1,
      "Args": [
        377   // USA first dozer
      ]
    }
  ]
}
{
  "TimeCode": 476,
  "OrderType": 1049, // Build
  "Number": 2,
  "UniqueOrders": 3,
  "Args": [
    {
      "Type": 0,
      "Count": 1,
      "Args": [
        1229    // USA Power Plant
      ]
    },
    {
      "Type": 6,
      "Count": 1,
      "Args": [
        {
          "X": 1144227816, // Probably broken
          "Y": 1158237722,
          "Z": 1106903024
        }
      ]
    },
    {
      "Type": 1,
      "Count": 1,
      "Args": [
        -0.7853982  // Angle??
      ]
    }
  ]
}
{
  "TimeCode": 718,
  "OrderType": 1001,  // Select
  "Number": 2,
  "UniqueOrders": 2,
  "Args": [
    {
      "Type": 2,
      "Count": 1,
      "Args": [
        true
      ]
    },
    {
      "Type": 3,
      "Count": 1,
      "Args": [
        383  // USA Dozer #2
      ]
    }
  ]
}
{
  "TimeCode": 752,
  "OrderType": 1068,  // Move to
  "Number": 2,
  "UniqueOrders": 1,
  "Args": [
    {
      "Type": 6,
      "Count": 1,
      "Args": [
        {
          "X": 1138571963, Where to move (should be nearish power plant, opposite side of command center)
          "Y": 1158466853,
          "Z": 1106903040
        }
      ]
    }
  ]
}
{
  "TimeCode": 796,
  "OrderType": 27,
  "Number": 2,
  "UniqueOrders": 0,
  "Args": []
}

```
