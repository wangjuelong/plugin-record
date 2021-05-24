package record

import (
	"encoding/json"
	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

var config struct {
	Path        string
	AutoPublish bool
	AutoRecord  bool
}

type RecordingInfo struct {
	ID        string      `json:"id"`
	Subscribe *Subscriber `json:"-"`         // 视频流
	Filepath  string      `json:"filepath"`  // 文件路径
	Recording bool        `json:"recording"` // 是否正在录制
}

var recordings sync.Map

type FlvFileInfo struct {
	Path     string
	Size     int64
	Duration uint32
}

func init() {
	InstallPlugin(&PluginConfig{
		Name:   "Record",
		Config: &config,
		Run:    run,
		HotConfig: map[string]func(interface{}){
			"AutoPublish": func(v interface{}) {
				config.AutoPublish = v.(bool)
			},
			"AutoRecord": func(v interface{}) {
				config.AutoRecord = v.(bool)
			},
		},
	})
}
func run() {
	go AddHook(HOOK_SUBSCRIBE, onSubscribe)
	go AddHook(HOOK_PUBLISH, onPublish)
	os.MkdirAll(config.Path, 0755)
	http.HandleFunc("/api/record/flv/list", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		if files, err := tree(config.Path, 0); err == nil {
			var bytes []byte
			if bytes, err = json.Marshal(files); err == nil {
				w.Write(bytes)
			} else {
				w.Write([]byte("{\"err\":\"" + err.Error() + "\"}"))
			}
		} else {
			w.Write([]byte("{\"err\":\"" + err.Error() + "\"}"))
		}
	})
	http.HandleFunc("/api/record/flv", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			if err := SaveFlv(streamPath, r.URL.Query().Get("append") == "true"); err != nil {
				w.Write([]byte(err.Error()))
			} else {
				w.Write([]byte("success"))
			}
		} else {
			w.Write([]byte("no streamPath"))
		}
	})

	http.HandleFunc("/api/record/status", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		var recordingInfo RecordingInfo
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			if stream, ok := recordings.Load(streamPath); ok {
				if output, ok := stream.(*RecordingInfo); ok {
					recordingInfo.Recording = output.Recording
					recordingInfo.Filepath = output.Filepath
					recordingInfo.ID = output.ID
				}
			}
		}
		data, _ := json.Marshal(recordingInfo)
		w.Write(data)
	})

	http.HandleFunc("/api/record/flv/stop", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			filePath := filepath.Join(config.Path, streamPath+".flv")
			if stream, ok := recordings.Load(filePath); ok {
				if output, ok := stream.(*RecordingInfo); ok {
					output.Subscribe.Close()
					output.Recording = false
					recordings.Store(filePath, output)
					w.Write([]byte("success"))
				} else {
					w.Write([]byte("no right stream"))
				}
			} else {
				w.Write([]byte("no query stream"))
			}
		} else {
			w.Write([]byte("no such stream"))
		}
	})
	http.HandleFunc("/api/record/flv/play", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			if err := PublishFlvFile(streamPath); err != nil {
				w.Write([]byte(err.Error()))
			} else {
				w.Write([]byte("success"))
			}
		} else {
			w.Write([]byte("no streamPath"))
		}
	})
	http.HandleFunc("/api/record/flv/delete", func(w http.ResponseWriter, r *http.Request) {
		CORS(w, r)
		if streamPath := r.URL.Query().Get("streamPath"); streamPath != "" {
			filePath := filepath.Join(config.Path, streamPath+".flv")
			if Exist(filePath) {
				if err := os.Remove(filePath); err != nil {
					w.Write([]byte(err.Error()))
				} else {
					w.Write([]byte("success"))
				}
			} else {
				w.Write([]byte("no such file"))
			}
		} else {
			w.Write([]byte("no streamPath"))
		}
	})
}
func onSubscribe(v interface{}) {
	s := v.(*Subscriber)
	if config.AutoPublish {
		filePath := filepath.Join(config.Path, s.StreamPath+".flv")
		if s.Publisher == nil && Exist(filePath) {
			PublishFlvFile(s.StreamPath)
		}
	}
}

func onPublish(v interface{}) {
	p := v.(*Stream)
	if config.AutoRecord {
		_ = SaveFlv(p.StreamPath, false)
	}
}

func tree(dstPath string, level int) (files []*FlvFileInfo, err error) {
	var dstF *os.File
	dstF, err = os.Open(dstPath)
	if err != nil {
		return
	}
	defer dstF.Close()
	fileInfo, err := dstF.Stat()
	if err != nil {
		return
	}
	if !fileInfo.IsDir() { //如果dstF是文件
		if path.Ext(fileInfo.Name()) == ".flv" {
			p := strings.TrimPrefix(dstPath, config.Path)
			p = strings.ReplaceAll(p, "\\", "/")
			files = append(files, &FlvFileInfo{
				Path:     strings.TrimPrefix(p, "/"),
				Size:     fileInfo.Size(),
				Duration: getDuration(dstF),
			})
		}
		return
	} else { //如果dstF是文件夹
		var dir []os.FileInfo
		dir, err = dstF.Readdir(0) //获取文件夹下各个文件或文件夹的fileInfo
		if err != nil {
			return
		}
		for _, fileInfo = range dir {
			var _files []*FlvFileInfo
			_files, err = tree(filepath.Join(dstPath, fileInfo.Name()), level+1)
			if err != nil {
				return
			}
			files = append(files, _files...)
		}
		return
	}

}
