package video

import (
	"io"
	"log"
	"os/exec"
	"sync"
)

type FFmpeg struct {
	cmd    *exec.Cmd
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
}

var ARGS = []string{"-threads", "4", "-re", "-i", "pipe:", "-an", "-f", "v4l2", "-fflags", "nobuffer", "-vcodec", "rawvideo", "-pix_fmt", "yuv420p", "/dev/video10"}

func NewFFmpegEncoder(wg *sync.WaitGroup) FFmpeg {
	cmd := exec.Command("ffmpeg", ARGS...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	wg.Add(1)
	go func(cmd *exec.Cmd) {
		err = cmd.Run()
		if err != nil {
			log.Println(err)
		}
		wg.Done()
	}(cmd)
	return FFmpeg{
		cmd:    cmd,
		Stdin:  stdin,
		Stdout: stdout,
	}
}
