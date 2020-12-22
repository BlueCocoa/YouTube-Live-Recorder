# YouTube-Live-Recorder

Automatically recording YouTube live streams!

### Dependencies
- [youtube-dl](https://github.com/ytdl-org/youtube-dl)
- [ffmpeg](https://ffmpeg.org/)

### Compile
```bash
go build -ldflags "-s -w"
```

### Config
```bash
cp config.example.json config.json
# edit config.json
```

#### Example
```json
{
  "channels": [                                # you can set multiple channels
    {
      "id": "UCoSrY_IQQVpmIRZ9Xf-y93g",        # copy their unique channel ID
      "save_to": "garwgura"                    # specify a directory for the recordings
    },
    {
      "id": "UC1opHUrw8rvnsadT-iGp7Cg",
      "save_to": "aqua"
    },
    {
      "id": "UC1DCedRgGHBdm81E1llLhOQ",
      "save_to": "pekora"
    }
  ],
  "APIKey": "YOUR YOUTUBE DATA V3 API KEY",    # you can apply your own YouTube Data V3 API Key at https://developers.google.com/youtube/v3/getting-started
  "python": "/usr/local/bin/python3",          # the path to your python interpreter which has `youtube_dl` package
  "log_level": "info",
  "query_interval": 3                          # query interval, you have 10,000 units per day and 1 single query costs 1 unit
}
```

### Usage
```bash
YouTube-Live-Recorder -conf config.json
```
