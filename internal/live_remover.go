package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

var jpgRe = regexp.MustCompile(`(?i)\.jp(e)*g`)

const liveExt = ".MOV"

func isJpeg(f string) bool {
	ext := filepath.Ext(f)
	return jpgRe.MatchString(ext)
}

func RemoveLiveVideos(dir string) error {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error when browsing file %v: %v", path, err)
		}
		if !info.IsDir() && isJpeg(path) {
			movFile := fmt.Sprintf("%v%v", path[0:strings.LastIndex(path, ".")], liveExt)
			if _, err := os.Stat(movFile); err == nil {
				if err = os.Remove(movFile); err != nil {
					log.Warn().Str(fileLogField, movFile).Msgf("error while removing file: %v", err)
				}
			}
		}
		return nil
	})
	return err
}
