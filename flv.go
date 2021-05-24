package record

import (
	"io"
	"os"
	"path/filepath"

	. "github.com/Monibuca/engine/v3"
	. "github.com/Monibuca/utils/v3"
	"github.com/Monibuca/utils/v3/codec"
)

func getDuration(file *os.File) uint32 {
	_, err := file.Seek(-4, io.SeekEnd)
	if err == nil {
		var tagSize uint32
		if tagSize, err = ReadByteToUint32(file, true); err == nil {
			_, err = file.Seek(-int64(tagSize)-4, io.SeekEnd)
			if err == nil {
				_, timestamp, _, err := codec.ReadFLVTag(file)
				if err == nil {
					return timestamp
				}
			}
		}
	}
	return 0
}

func SaveFlv(streamPath string, append bool) error {
	flag := os.O_CREATE
	if append {
		flag = flag | os.O_RDWR | os.O_APPEND
	} else {
		flag = flag | os.O_TRUNC | os.O_WRONLY
	}
	filePath, err := filepath.Abs(filepath.Join(config.Path, streamPath+".flv"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(filePath, flag, 0755)
	if err != nil {
		return err
	}
	// return avformat.WriteFLVTag(file, packet)
	p := Subscriber{
		ID:   filePath,
		Type: "FlvRecord",
	}
	var offsetTime uint32
	if append {
		fileInfo, err := file.Stat()
		if err == nil && fileInfo.Size() == 0 {
			_, err = file.Write(codec.FLVHeader)
		}
		offsetTime = getDuration(file)
		file.Seek(0, io.SeekEnd)
	} else {
		_, err = file.Write(codec.FLVHeader)
	}
	if err == nil {
		recordings.Store(filePath, RecordingInfo{
			ID:        streamPath,
			Subscribe: &p,
			Filepath:  filePath,
			Recording: true,
		})
		if err := p.Subscribe(streamPath); err == nil {
			at, vt := p.WaitAudioTrack("aac", "pcma", "pcmu"), p.WaitVideoTrack("h264")
			tag0 := at.RtmpTag[0]
			p.OnAudio = func(audio AudioPack) {
				if !append && tag0>>4 == 10 { //AAC格式需要发送AAC头
					codec.WriteFLVTag(file, codec.FLV_TAG_TYPE_AUDIO, 0, at.RtmpTag)
				}
				codec.WriteFLVTag(file, codec.FLV_TAG_TYPE_AUDIO, audio.Timestamp+offsetTime, audio.ToRTMPTag(tag0))
				p.OnAudio = func(audio AudioPack) {
					codec.WriteFLVTag(file, codec.FLV_TAG_TYPE_AUDIO, audio.Timestamp+offsetTime, audio.ToRTMPTag(tag0))
				}
			}
			p.OnVideo = func(video VideoPack) {
				if !append {
					codec.WriteFLVTag(file, codec.FLV_TAG_TYPE_VIDEO, 0, vt.RtmpTag)
				}
				codec.WriteFLVTag(file, codec.FLV_TAG_TYPE_VIDEO, video.Timestamp+offsetTime, video.ToRTMPTag())
				p.OnVideo = func(video VideoPack) {
					codec.WriteFLVTag(file, codec.FLV_TAG_TYPE_VIDEO, video.Timestamp+offsetTime, video.ToRTMPTag())
				}
			}
			go func() {
				p.Play(at, vt)
				file.Close()
			}()
		}

	} else {
		file.Close()
	}
	return err
}
