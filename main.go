package main

import (
	config "./internal/config"
	"encoding/json"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type NewLiveStreamEvent struct {
	ChannelInfo config.Channel
	LiveItemInfo LiveItem
}

type YouTubeDLProcess struct {
	Event NewLiveStreamEvent
	Process *exec.Cmd
	StartTime time.Time
}

type YouTubeDLProcessDidTerminate struct {
	Event NewLiveStreamEvent
	Process *exec.Cmd
	TerminateTime time.Time
	Err error
}

type LiveItemID struct {
	Kind string `json:"kind"`
	VideoID string `json:"videoId"`
}

type LiveItem struct {
	Kind string `json:"kind"`
	ETag string `json:"etag"`
	ID LiveItemID `json:"id"`
	Snippet map[string]interface{} `json:"snippet"`
}

type Response struct {
	Kind string `json:"kind"`
	ETag string `json:"etag"`
	RegionCode string `json:"regionCode"`
	PageInfo map[string]int `json:"pageInfo"`
	Items []LiveItem `json:"items"`
}

var configPath string
var cfg config.Config
var channelReq []*http.Request
var newYouTubeDLProcess chan YouTubeDLProcess
var youTubeDLProcessDidTerminate chan YouTubeDLProcessDidTerminate
var confPtr *string
func init() {
	confPtr = flag.String("conf", "config.json", "Path to the config file")
	reloadConfig()
}

func reloadConfig() {
	configPath = *confPtr

	// read config
	cfg = config.ReadConfig(configPath)

	// output to stdout instead of the default stderr
	// can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	log.SetLevel(log.InfoLevel)
	logLevel := strings.ToLower(cfg.LogLevel)
	switch logLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	default:
		log.Errorf("Unknown log level '%s', will set to Info level", logLevel)
		log.SetLevel(log.InfoLevel)
	}
	log.Debugln("cfg:", cfg)

	channelReq = make([]*http.Request, len(cfg.Channels))
	for index, channel := range cfg.Channels {
		url := fmt.Sprintf("https://www.googleapis.com/youtube/v3/search?part=snippet&channelId=%s&type=video&eventType=live&key=%s", channel.ID, cfg.APIKey)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatalf("cannot new httpRequest: %v\n", err)
			os.Exit(-1)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_0_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.36")
		req.Header.Set("Host", "www.youtube.com")
		channelReq[index] = req

		err = os.MkdirAll(channel.SaveTo, os.ModePerm)
		if err != nil {
			log.Fatalf("cannot create directory at: %s: %v\n", channel.SaveTo, err)
			os.Exit(-1)
		}
	}
}

func manageYouTubeDLProcess() {
	processes := make(map[string]YouTubeDLProcess)
	for {
		select {
		case process := <-newYouTubeDLProcess:
			processes[process.Event.LiveItemInfo.ID.VideoID] = process
			log.Infof("currently %d YouTube processes running", len(processes))
		case terminate := <-youTubeDLProcessDidTerminate:
			if proc, ok := processes[terminate.Event.LiveItemInfo.ID.VideoID]; ok {
				err := terminate.Err
				videoID := proc.Event.LiveItemInfo.ID.VideoID
				youtubeVideoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
				if err != nil {
					log.Errorf("error occurred while recording %s: %v", youtubeVideoURL, err)
				} else {
					log.Infof("live stream has ended: %s", youtubeVideoURL)
				}
				delete(processes, videoID)
			}
		}
	}
}

func main() {
	newLiveStreamChan := make(chan NewLiveStreamEvent)
	newYouTubeDLProcess = make(chan YouTubeDLProcess)
	youTubeDLProcessDidTerminate = make(chan YouTubeDLProcessDidTerminate)

	go manageYouTubeDLProcess()
	go func(){
		recording := make(map[string]bool)
		for {
			newLive := <-newLiveStreamChan
			videoID := newLive.LiveItemInfo.ID.VideoID
			if _, ok := recording[newLive.LiveItemInfo.ID.VideoID]; !ok {
				recording[newLive.LiveItemInfo.ID.VideoID] = true
				youtubeVideoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

				args := []string{"-m", "youtube_dl", youtubeVideoURL}
				cmd := exec.Command( cfg.Python, args... )
				cmd.Dir = newLive.ChannelInfo.SaveTo
				err := cmd.Start()
				if err != nil {
					log.Errorln(err)
				}
				pid := cmd.Process.Pid
				log.Infof("start recording live: %s [FFmpeg: %d]", youtubeVideoURL, pid)
				newYouTubeDLProcess <- YouTubeDLProcess{
					Event: newLive,
					Process: cmd,
					StartTime: time.Now(),
				}
				go func() {
					err = cmd.Wait()
					youTubeDLProcessDidTerminate <- YouTubeDLProcessDidTerminate{
						Event:         newLive,
						Process:       cmd,
						TerminateTime: time.Now(),
						Err:           err,
					}
				}()
			}
		}
	}()

	go func() {
		client := &http.Client{}
		for {
			for channelIndex, req := range channelReq {
				log.Debugf("querying live status for channel: %s\n", cfg.Channels[channelIndex].ID)

				resp, err := client.Do(req)
				if err != nil {
					log.Errorf("cannot query live info for channel: https://www.youtube.com/channel/%s: %v\n", cfg.Channels[channelIndex].ID, err)
					continue
				}

				defer resp.Body.Close()
				html, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Errorf("cannot load YouTube API response for channel: https://www.youtube.com/channel/%s: %v\n", cfg.Channels[channelIndex].ID, err)
					continue
				}

				var response Response
				err = json.Unmarshal(html, &response)
				if err != nil {
					log.Errorf("cannot parse YouTube API response for channel: https://www.youtube.com/channel/%s: %v\n", cfg.Channels[channelIndex].ID, err)
					continue
				}

				numLiveStreams := len(response.Items)
				if numLiveStreams > 0 {
					log.Debugf(" channel %s has %d live stream(s)\n", cfg.Channels[channelIndex].ID, numLiveStreams)
					for _, item := range response.Items  {
						event := NewLiveStreamEvent {
							ChannelInfo: cfg.Channels[channelIndex],
							LiveItemInfo: item,
						}
						newLiveStreamChan <- event
					}
				} else {
					log.Debugf("channel %s has 0 live stream", cfg.Channels[channelIndex].ID)
				}
			}

			time.Sleep(time.Duration(cfg.QueryInterval) * time.Minute)
			reloadConfig()
		}
	}()

	waitForever := make(chan int)
	<-waitForever
}
