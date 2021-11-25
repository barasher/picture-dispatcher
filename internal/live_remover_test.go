package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsJpeg(t *testing.T) {
	var tcs = []struct {
		input     string
		expIsJpeg bool
	}{
		{"/tmp/a.jPg", true},
		{"/tmp/a.jpEg", true},
		{"a.jpEg", true},
		{"noExt", false},
		{"/tmp/a.txt", false},
		{"a.txt", false},
	}

	for _, tc := range tcs {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expIsJpeg, isJpeg(tc.input))
		})
	}
}

func TestRemoveLiveVideos(t *testing.T) {
	tmpDir := t.TempDir()

	liveJpgFile := filepath.Join(tmpDir, "a.jpg")
	copy("../testdata/input/20190404_131804.jpg", liveJpgFile)
	liveMovFile := filepath.Join(tmpDir, "a.MOV")
	copy("../testdata/input/20190404_131804.jpg", liveMovFile)

	subDir := "sub"
	assert.Nil(t, os.Mkdir(filepath.Join(tmpDir, subDir), 0777))
	subLiveJpgFile := filepath.Join(tmpDir, subDir, "d.jpg")
	copy("../testdata/input/20190404_131804.jpg", subLiveJpgFile)
	subLiveMovFile := filepath.Join(tmpDir, subDir, "d.MOV")
	copy("../testdata/input/20190404_131804.jpg", subLiveMovFile)

	singleJpgFile := filepath.Join(tmpDir, "b.jpg")
	copy("../testdata/input/20190404_131804.jpg", singleJpgFile)
	nonJpgFile := filepath.Join(tmpDir, "c.txt")
	copy("../testdata/input/20190404_131804.jpg", nonJpgFile)
	singleMovFile := filepath.Join(tmpDir, "e.MOV")
	copy("../testdata/input/20190404_131804.jpg", singleMovFile)

	assert.Nil(t, RemoveLiveVideos(tmpDir))

	checkExist(t, liveJpgFile, true)
	checkExist(t, liveMovFile, false)
	checkExist(t, subLiveJpgFile, true)
	checkExist(t, subLiveMovFile, false)
	checkExist(t, singleJpgFile, true)
	checkExist(t, nonJpgFile, true)
	checkExist(t, singleMovFile, true)

}
