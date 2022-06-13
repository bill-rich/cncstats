# cncstats


## How to run
The default behavior for cncstats is to run as a webserver that will parse replays and return the replay data in JSON format. You can override this by setting the `LOCAL` env variable. You also need to provide the INI location as a env var. eg CNC_INI=./inizh/Data/INI/.

For Zero Hour local:
```
LOCAL=true CNC_INI=./inizh/Data/INI go run . ~/Downloads/replay.rep
```

For BMFE (Still incomplete and hacky. Must use BMFE branch):
```    
LOCAL=true go run . ~/Downloads/replay.rep     
```

For easy CLI viewing, pipe the command into jq:
```    
LOCAL=true go run . ~/Downloads/replay.rep | jq . | less
```



## Useful commands
Filter by playerID, and remove checksum, deselects, and camera movements.
`jq '.Body[] | select(.Number==2 and .OrderType != 1095 and .OrderType != 1092 and .OrderType != 1003)' | less`

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
